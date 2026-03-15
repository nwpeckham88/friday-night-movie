package logic

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/user/friday-night-movie/pkg/config"
	"github.com/user/friday-night-movie/pkg/db"
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
		updateStatus("Syncing cinematic records...", true)
		jellyfinMovies, err = jClient.GetMovies("")
		if err != nil {
			return nil, fmt.Errorf("failed to fetch jellyfin movies: %w", err)
		}
		radarrMovies, err = rClient.GetMovies()
		if err != nil {
			fmt.Printf("Warning: Failed to fetch Radarr movies: %v\n", err)
		}

		// Sync to local DB for high-speed lookup
		var items []db.DBSuggestion
		for _, m := range jellyfinMovies {
			mid := 0
			if m.ProviderIds.Tmdb != "" {
				fmt.Sscanf(m.ProviderIds.Tmdb, "%d", &mid)
			}
			items = append(items, db.DBSuggestion{
				TMDBID:    mid,
				Title:     m.Name,
				Reasoning: strings.Join(m.Genres, ", "), // Store genres in reasoning field of library table
			})
		}
		db.SyncLibrary(items)

		// Update cache
		cacheMutex.Lock()
		jellyfinCache = jellyfinMovies
		radarrCache = radarrMovies
		cacheTime = time.Now()
		cacheMutex.Unlock()
	}

	existingTitles := make(map[string]bool)
	existingIDs := make(map[int]bool)

	normalize := func(s string) string {
		return strings.ToLower(strings.TrimSpace(s))
	}

	for _, m := range jellyfinMovies {
		existingTitles[normalize(m.Name)] = true
		if m.ProviderIds.Tmdb != "" {
			var mid int
			if _, err := fmt.Sscanf(m.ProviderIds.Tmdb, "%d", &mid); err == nil {
				existingIDs[mid] = true
			}
		}
	}
	for _, m := range radarrMovies {
		existingTitles[normalize(m.Title)] = true
		if m.TmdbId > 0 {
			existingIDs[m.TmdbId] = true
		}
	}

	// NEW: Add rejected movies to existing lists
	state := config.GetState()
	for _, r := range state.RejectedMovies {
		// rejected format is usually "Title (ID)"
		existingTitles[normalize(r)] = true
		if idx := strings.LastIndex(r, "("); idx != -1 {
			titlePart := normalize(r[:idx])
			existingTitles[titlePart] = true
			var rid int
			if _, err := fmt.Sscanf(r[idx+1:], "%d", &rid); err == nil {
				existingIDs[rid] = true
			}
		}
	}

	// NEW: Add past suggestions to existing lists to prevent repetition
	pastSuggestions, _ := db.GetSuggestions()
	for _, ps := range pastSuggestions {
		existingIDs[ps.TMDBID] = true
		existingTitles[normalize(ps.Title)] = true
	}

	maxRetries := 2
	var selectedMovie *discovery.TMDBMovie
	var failedSuggestions []string

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// 1. Weekly Cinema Research context
		updateStatus("COMMUNING WITH ARCHIVES...", true)
		today := time.Now().Format("Jan 02, 2006")
		weeklyContext, _ := discovery.GetWeeklyCinemaContext(provider, today)
		if weeklyContext != "" {
			updateStatus("CINEMATIC SIGNAL IDENTIFIED...", true)
		}

		var historyStrings []string

		// 2. Genres Summary (Top 10)
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

		// 5. Discover via Expert LLM
		updateStatus("AGENT IS THINKING...", true)
		suggestions, err := provider.DiscoverMovie(historyStrings, state.TasteProfile, state.RejectedMovies, failedSuggestions, weeklyContext, func(msg string) {
			updateStatus("PROCESS: "+msg, true)
		})
		if err != nil {
			return nil, fmt.Errorf("expert discovery failed: %w", err)
		}

		for _, suggestion := range suggestions {
			updateStatus(fmt.Sprintf("Resolving '%s' (%d) on TMDB...", suggestion.Title, suggestion.Year), true)
			movie, err := tClient.SearchMovie(suggestion.Title, suggestion.Year)
			if err != nil {
				fmt.Printf("TMDB resolution failed for '%s', skipping... error: %v\n", suggestion.Title, err)
				failedSuggestions = append(failedSuggestions, suggestion.Title)
				continue
			}

			// 5. Check Language (Strict Mode)
			if cfg.StrictLanguage && cfg.PreferredLanguage != "" {
				if !strings.EqualFold(movie.OriginalLanguage, cfg.PreferredLanguage) {
					fmt.Printf("Expert suggested '%s' in language '%s', but strict mode requires '%s'. Skipping...\n", movie.Title, movie.OriginalLanguage, cfg.PreferredLanguage)
					failedSuggestions = append(failedSuggestions, movie.Title)
					continue
				}
			}

			// 6. Check if it's already in our library
			inLibrary := db.IsMovieInLibrary(movie.ID)
			if inLibrary && !cfg.SuggestInLibrary {
				fmt.Printf("Expert suggested '%s' (ID: %d) which is in library, skipping...\n", movie.Title, movie.ID)
				failedSuggestions = append(failedSuggestions, movie.Title)
				continue
			} else if inLibrary && cfg.SuggestInLibrary {
				updateStatus(fmt.Sprintf("RE-WATCH PATH IDENTIFIED: '%s' is in your library!", movie.Title), true)
			}

			if existingIDs[movie.ID] || existingTitles[normalize(movie.Title)] {
				// Only skip if NOT in library or re-watch disabled
				if !inLibrary || !cfg.SuggestInLibrary {
					fmt.Printf("Expert suggested duplicate '%s' (ID: %d), skipping...\n", movie.Title, movie.ID)
					failedSuggestions = append(failedSuggestions, movie.Title)
					continue
				}
			}

			// 7. Check Minimum Rating
			if movie.VoteAverage < cfg.MinRating {
				fmt.Printf("Expert suggested '%s' but rating %.1f is below minimum %.1f, skipping...\n", movie.Title, movie.VoteAverage, cfg.MinRating)
				failedSuggestions = append(failedSuggestions, movie.Title)
				continue
			}

			// Found a good one!
			selectedMovie = movie
			selectedMovie.Reasoning = suggestion.Reasoning
			selectedMovie.PathTheme = suggestion.PathTheme
			
			// New: Fetch Trailer Key
			trailerKey, err := tClient.GetMovieTrailer(movie.ID)
			if err == nil && trailerKey != "" {
				selectedMovie.TrailerKey = trailerKey
			}

			// Capture reasoning and path theme from suggestion
			// We can't directly add these to discovery.TMDBMovie as it's a TMDB struct,
			// but we can return them or handle them in main.go
			// Actually, let's keep selectedMovie as is, but we'll need to pass reasoning back.

			// Update persistent state immediately if it's a suggestions search
			// (or wait for the caller to handle it) - engine.go doesn't usually save state, main.go does.
			// However, DiscoverNewMovie is called by main.go handlers.
			
			break
		}

		if selectedMovie != nil {
			break
		}
		fmt.Printf("No valid movies found in this batch of experts, retrying (attempt %d/3)...\n", attempt+1)
	}

	return selectedMovie, nil
}

// AddMovieToRadarr adds a resolved TMDB movie to Radarr
func AddMovieToRadarr(movie *discovery.TMDBMovie, rClient *downloader.Client, updateStatus func(string, bool)) error {
	updateStatus(fmt.Sprintf("Adding '%s' to Radarr...", movie.Title), true)

	cfg := config.GetConfig()
	year := time.Now().Year()
	if len(movie.ReleaseDate) >= 4 {
		fmt.Sscanf(movie.ReleaseDate[:4], "%d", &year)
	}

	qProfile := cfg.RadarrQualityProfileID
	if qProfile == 0 {
		qProfile = 1
	}

	addPayload := map[string]interface{}{
		"title":            movie.Title,
		"tmdbId":           movie.ID,
		"year":             year,
		"qualityProfileId": qProfile,
		"monitored":        true,
		"rootFolderPath":   "/data/media/movies",
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
