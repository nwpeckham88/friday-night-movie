package logic

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/user/friday-night-movie/pkg/discovery"
	"github.com/user/friday-night-movie/pkg/downloader"
	"github.com/user/friday-night-movie/pkg/media"
)

// RunFridayNightRoutine orchestrates finding a new movie and sending it to Radarr (Auto-add)
func RunFridayNightRoutine(
	jClient *media.JellyfinClient,
	tClient *discovery.TMDBClient,
	gClient *discovery.GeminiClient,
	rClient *downloader.Client,
	updateStatus func(string, bool),
) (*discovery.TMDBMovie, error) {
	movie, err := DiscoverNewMovie(jClient, tClient, gClient, rClient, updateStatus)
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

// DiscoverNewMovie finds a movie via Gemini but DOES NOT add it to Radarr
func DiscoverNewMovie(
	jClient *media.JellyfinClient,
	tClient *discovery.TMDBClient,
	gClient *discovery.GeminiClient,
	rClient *downloader.Client,
	updateStatus func(string, bool),
) (*discovery.TMDBMovie, error) {
	updateStatus("Fetching existing library...", true)

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
		// 1. Get existing movies from Jellyfin
		jellyfinMovies, err = jClient.GetMovies("")
		if err != nil {
			return nil, fmt.Errorf("failed to fetch jellyfin movies: %w", err)
		}

		// 2. Get existing movies from Radarr
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

	// 2. Populate lists
	for _, m := range radarrMovies {
		existingTitles[m.Title] = true
		if m.TmdbId > 0 {
			existingIDs[m.TmdbId] = true
		}
	}

	// 3. Discovery Loop (with Retries)
	var selectedMovie *discovery.TMDBMovie
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		retryMsg := ""
		if attempt > 1 {
			retryMsg = fmt.Sprintf(" (Attempt %d/%d)", attempt, maxRetries)
		}
		updateStatus("Gemini Thinking & Searching..."+retryMsg, true)

		// Prepare history (Optimized for tokens)
		var historyStrings []string

		// 1. Top Genres Summarization
		genreCounts := make(map[string]int)
		for _, m := range jellyfinMovies {
			for _, g := range m.Genres {
				genreCounts[g]++
			}
		}
		// Sort genres by count (simple manual sort for top 10)
		type genreStat struct {
			Name  string
			Count int
		}
		var topGenres []genreStat
		for name, count := range genreCounts {
			topGenres = append(topGenres, genreStat{name, count})
		}
		sort.Slice(topGenres, func(i, j int) bool {
			return topGenres[i].Count > topGenres[j].Count
		})
		
		genreSummary := ""
		for i, g := range topGenres {
			if i >= 10 { break }
			genreSummary += fmt.Sprintf("%s (%d), ", g.Name, g.Count)
		}
		if genreSummary != "" {
			historyStrings = append(historyStrings, "User's favorite genres: "+strings.TrimSuffix(genreSummary, ", "))
		}

		// 2. Sliding Window: Only send the 40 most recent titles as literal examples
		historyStrings = append(historyStrings, "Recently watched/queued movies (for style reference):")
		limit := 40
		count := 0
		// Iterate backwards to get latest (assuming Jellyfin order or just taking first 40 for now)
		for i := len(jellyfinMovies)-1; i >= 0 && count < limit; i-- {
			historyStrings = append(historyStrings, jellyfinMovies[i].Name)
			count++
		}

		suggestion, err := gClient.DiscoverMovie(historyStrings)
		if err != nil {
			return nil, fmt.Errorf("gemini discovery failed: %w", err)
		}

		updateStatus(fmt.Sprintf("Resolving '%s' on TMDB...", suggestion.Title), true)
		movie, err := tClient.SearchMovie(suggestion.SearchQuery)
		if err != nil {
			if attempt < maxRetries {
				fmt.Printf("TMDB resolution failed, retrying... error: %v\n", err)
				continue
			}
			return nil, fmt.Errorf("failed to sync gemini result with tmdb (%s): %w", suggestion.SearchQuery, err)
		}

		// 5. Check if it's already in our library (Robust ID + Title check)
		if existingIDs[movie.ID] || existingTitles[movie.Title] {
			if attempt < maxRetries {
				fmt.Printf("Gemini suggested duplicate '%s' (ID: %d), retrying...\n", movie.Title, movie.ID)
				existingIDs[movie.ID] = true
				existingTitles[movie.Title] = true
				continue
			}
			return nil, fmt.Errorf("gemini suggested a movie we already have after %d attempts: %s", maxRetries, movie.Title)
		}

		selectedMovie = movie
		break
	}

	return selectedMovie, nil
}

// AddMovieToRadarr adds a resolved TMDB movie to Radarr
func AddMovieToRadarr(movie *discovery.TMDBMovie, rClient *downloader.Client, updateStatus func(string, bool)) error {
	updateStatus(fmt.Sprintf("Adding '%s' to Radarr...", movie.Title), true)

	// Parse year from ReleaseDate (YYYY-MM-DD)
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
