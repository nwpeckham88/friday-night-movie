package logic

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/user/friday-night-movie/pkg/config"
	"github.com/user/friday-night-movie/pkg/discovery"
	"github.com/user/friday-night-movie/pkg/downloader"
	"github.com/user/friday-night-movie/pkg/media"
)

// RunFridayNightRoutine orchestrates finding a new movie and sending it to Radarr (Auto-add)
func RunFridayNightRoutine(cfg config.AppConfig, jClient *media.JellyfinClient, tClient *discovery.TMDBClient, rClient *downloader.Client, provider discovery.MovieDiscoverer, updateStatus func(string, bool)) (*discovery.TMDBMovie, error) {
	movie, err := DiscoverNewMovie(cfg, jClient, rClient, tClient, provider, updateStatus, false)
	if err != nil {
		return nil, err
	}

	if err := AddMovieToRadarr(movie, rClient, updateStatus); err != nil {
		return nil, err
	}

	return movie, nil
}

var (
	cacheMutex    sync.RWMutex
	jellyfinCache []media.JellyfinItem
	radarrCache   []downloader.Movie
	cacheTime     time.Time
)

const cacheTTL = 2 * time.Minute

// DiscoverNewMovie orchestrates the entire discovery flow: library -> expert -> tmdb -> radarr
func DiscoverNewMovie(cfg config.AppConfig, jClient *media.JellyfinClient, rClient *downloader.Client, tClient *discovery.TMDBClient, provider discovery.MovieDiscoverer, updateStatus func(string, bool), discoverOnly bool) (*discovery.TMDBMovie, error) {
	updateStatus("Fetching library & cinema history...", true)

	cacheMutex.RLock()
	useCache := time.Since(cacheTime) < cacheTTL && len(jellyfinCache) > 0
	cacheMutex.RUnlock()

	var jellyfinMovies []media.JellyfinItem
	var radarrMovies []downloader.Movie
	var err error

	if useCache {
		cacheMutex.RLock()
		jellyfinMovies = jellyfinCache
		radarrMovies = radarrCache
		cacheMutex.RUnlock()
		fmt.Println("Using in-memory library cache")
	} else {
		jellyfinMovies, err = jClient.GetMovies("")
		if err != nil {
			return nil, fmt.Errorf("failed to fetch jellyfin movies: %w", err)
		}
		radarrMovies, err = rClient.GetMovies()
		if err != nil {
			fmt.Printf("Warning: Failed to fetch Radarr movies: %v\n", err)
		}
		// Update cache
		cacheMutex.Lock()
		jellyfinCache = jellyfinMovies
		radarrCache = radarrMovies
		cacheTime = time.Now()
		cacheMutex.Unlock()
	}

	existingTitles := make(map[string]bool)
	existingIDs := make(map[int]bool)

	for _, m := range jellyfinMovies {
		existingTitles[m.Name] = true
		if m.ProviderIds.Tmdb != "" {
			var mid int
			if _, err := fmt.Sscanf(m.ProviderIds.Tmdb, "%d", &mid); err == nil {
				existingIDs[mid] = true
			}
		}
	}
	for _, m := range radarrMovies {
		existingTitles[m.Title] = true
		if m.TmdbId > 0 {
			existingIDs[m.TmdbId] = true
		}
	}

	maxRetries := 2
	var selectedMovie *discovery.TMDBMovie

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Calculate history strings for LLM
		var historyStrings []string
		
		// 1. Genres Summary (Top 10)
		genreSummary := jClient.GetTopGenres(10)
		if genreSummary != "" {
			historyStrings = append(historyStrings, "User's favorite genres: "+strings.TrimSuffix(genreSummary, ", "))
		}

		// 2. Sliding Window: Only send the 40 most recent titles as literal examples
		historyStrings = append(historyStrings, "Recently watched/queued movies (for style reference):")
		limit := 40
		count := 0
		for i := len(jellyfinMovies)-1; i >= 0 && count < limit; i-- {
			historyStrings = append(historyStrings, jellyfinMovies[i].Name)
			count++
		}

		// 3. TMDB Freshness Bridge (Grounding for non-search models)
		isGemini := true
		providerType := os.Getenv("LLM_PROVIDER")
		if providerType != "" && providerType != "gemini" {
			isGemini = false
		}

		if !isGemini {
			updateStatus("Grounding Expert with latest cinema releases...", true)
			trending, err := tClient.GetTrendingMovies()
			if err == nil && len(trending) > 0 {
				historyStrings = append(historyStrings, "\nCURRENT CINEMA CONTEXT (Released 2024-2026):")
				for i, m := range trending {
					if i >= 15 { break }
					historyStrings = append(historyStrings, fmt.Sprintf("%s (%s)", m.Title, m.ReleaseDate))
				}
				historyStrings = append(historyStrings, "Instructions: You may recommend from this list IF it fits the user, or use it to inform your expertise on recent trends.")
			}
		}

		// 4. Discover via Expert LLM
		state := config.GetState()
		suggestion, err := provider.DiscoverMovie(historyStrings, state.TasteProfile, state.RejectedMovies, func(msg string) {
			updateStatus(msg, true)
		})
		if err != nil {
			return nil, fmt.Errorf("expert discovery failed: %w", err)
		}

		updateStatus(fmt.Sprintf("Resolving '%s' (%d) on TMDB...", suggestion.Title, suggestion.Year), true)
		movie, err := tClient.SearchMovie(suggestion.Title, suggestion.Year)
		if err != nil {
			if attempt < maxRetries {
				fmt.Printf("TMDB resolution failed, retrying... error: %v\n", err)
				continue
			}
			return nil, fmt.Errorf("failed to sync expert result with tmdb (%s): %w", suggestion.Title, err)
		}

		// 5. Check Language (Strict Mode)
		if cfg.StrictLanguage && cfg.PreferredLanguage != "" {
			if !strings.EqualFold(movie.OriginalLanguage, cfg.PreferredLanguage) {
				if attempt < maxRetries {
					fmt.Printf("Expert suggested '%s' in language '%s', but strict mode requires '%s'. Retrying...\n", movie.Title, movie.OriginalLanguage, cfg.PreferredLanguage)
					continue
				}
				return nil, fmt.Errorf("expert failed to suggest a movie in %s after %d attempts", cfg.PreferredLanguage, maxRetries)
			}
		}

		// 6. Check if it's already in our library
		if existingIDs[movie.ID] || existingTitles[movie.Title] {
			if attempt < maxRetries {
				fmt.Printf("Expert suggested duplicate '%s' (ID: %d), retrying...\n", movie.Title, movie.ID)
				existingIDs[movie.ID] = true
				existingTitles[movie.Title] = true
				continue
			}
			return nil, fmt.Errorf("expert suggested a movie we already have after %d attempts: %s", maxRetries, movie.Title)
		}

		selectedMovie = movie
		break
	}

	return selectedMovie, nil
}

// AddMovieToRadarr adds a resolved TMDB movie to Radarr
func AddMovieToRadarr(movie *discovery.TMDBMovie, rClient *downloader.Client, updateStatus func(string, bool)) error {
	updateStatus(fmt.Sprintf("Adding '%s' to Radarr...", movie.Title), true)

	year := time.Now().Year()
	if len(movie.ReleaseDate) >= 4 {
		fmt.Sscanf(movie.ReleaseDate[:4], "%d", &year)
	}

	addPayload := map[string]interface{}{
		"title":            movie.Title,
		"tmdbId":           movie.ID,
		"year":             year,
		"qualityProfileId": 1,
		"monitored":        true,
		"rootFolderPath":   "/movies",
		"addOptions": map[string]bool{
			"searchForMovie": true,
		},
	}

	if err := rClient.AddMovie(addPayload); err != nil {
		return fmt.Errorf("failed to add movie to radarr: %w", err)
	}

	updateStatus("Successfully added to queue!", false)
	return nil
}
