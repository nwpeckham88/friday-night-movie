package logic

import (
	"fmt"
	"strings"
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

// DiscoverNewMovie finds a movie via Gemini but DOES NOT add it to Radarr
func DiscoverNewMovie(
	jClient *media.JellyfinClient,
	tClient *discovery.TMDBClient,
	gClient *discovery.GeminiClient,
	rClient *downloader.Client,
	updateStatus func(string, bool),
) (*discovery.TMDBMovie, error) {
	updateStatus("Fetching existing library...", true)

	// 1. Get existing movies from Jellyfin
	jellyfinMovies, err := jClient.GetMovies("")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch jellyfin movies: %w", err)
	}
	existingTitles := make(map[string]bool)
	for _, m := range jellyfinMovies {
		existingTitles[m.Name] = true
	}

	// 2. Get existing movies from Radarr
	radarrMovies, err := rClient.GetMovies()
	if err != nil {
		fmt.Printf("Warning: Failed to fetch Radarr movies: %v\n", err)
	}
	for _, m := range radarrMovies {
		existingTitles[m.Title] = true
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

		// Prepare history
		var historyStrings []string
		for _, m := range jellyfinMovies {
			genres := strings.Join(m.Genres, "/")
			historyStrings = append(historyStrings, fmt.Sprintf("%s (Genres: %s)", m.Name, genres))
		}

		for title := range existingTitles {
			found := false
			for _, m := range jellyfinMovies {
				if m.Name == title {
					found = true
					break
				}
			}
			if !found {
				historyStrings = append(historyStrings, title)
			}
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

		// 5. Check if it's already in our library
		if existingTitles[movie.Title] {
			if attempt < maxRetries {
				fmt.Printf("Gemini suggested duplicate '%s', retrying...\n", movie.Title)
				existingTitles[movie.Title] = true // Ensure it stays in the "blacklist" for next retry
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

	addPayload := map[string]interface{}{
		"title":            movie.Title,
		"titleSlug":        fmt.Sprintf("%d", movie.ID),
		"tmdbId":           movie.ID,
		"year":             time.Now().Year(), // ideally parse from ReleaseDate if possible
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
