package logic

import (
	"fmt"
	"time"

	"github.com/user/friday-night-movie/pkg/discovery"
	"github.com/user/friday-night-movie/pkg/downloader"
	"github.com/user/friday-night-movie/pkg/media"
)

// RunFridayNightRoutine orchestrates finding a new movie and sending it to Radarr
func RunFridayNightRoutine(jClient *media.JellyfinClient, tClient *discovery.TMDBClient, gClient *discovery.GeminiClient, rClient *downloader.Client) (*discovery.TMDBMovie, error) {
	fmt.Println("Running Friday Night Routine...")

	// 1. Get existing movies from Jellyfin
	fmt.Println("Fetching existing Jellyfin library...")
	jellyfinMovies, err := jClient.GetMovies("")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch jellyfin movies: %w", err)
	}
	existingTitles := make(map[string]bool)
	for _, m := range jellyfinMovies {
		existingTitles[m.Name] = true
	}

	// 2. Get existing movies from Radarr (to ensure we don't try to add something already queued/downloaded)
	fmt.Println("Fetching Radarr library...")
	radarrMovies, err := rClient.GetMovies()
	if err != nil {
		fmt.Printf("Warning: Failed to fetch Radarr movies: %v\n", err)
	}
	for _, m := range radarrMovies {
		existingTitles[m.Title] = true
	}

	// 3. Discover new movie via Gemini LLM Think & Search
	fmt.Println("Discovering movies via Gemini Intelligence...")
	
	// Prepare history context
	var historyStrings []string
	for title := range existingTitles {
		historyStrings = append(historyStrings, title)
	}

	suggestedTitle, err := gClient.DiscoverMovie(historyStrings)
	if err != nil {
		return nil, fmt.Errorf("gemini discovery failed: %w", err)
	}

	fmt.Printf("Gemini Suggested: %s\n", suggestedTitle)

	// 4. Resolve the title to a TMDB ID
	fmt.Println("Resolving title to TMDB ID...")
	selectedMovie, err := tClient.SearchMovie(suggestedTitle)
	if err != nil {
		return nil, fmt.Errorf("failed to sync gemini result with tmdb (%s): %w", suggestedTitle, err)
	}

	// 5. Check if it's already in our library (Jellyfin/Radarr) by the resolved name
	if existingTitles[selectedMovie.Title] {
		return nil, fmt.Errorf("gemini suggested a movie we already have: %s", selectedMovie.Title)
	}

	fmt.Printf("Selected Movie: %s (TMDB ID: %d, Rating: %.1f)\n", selectedMovie.Title, selectedMovie.ID, selectedMovie.VoteAverage)

	// 6. Add to Radarr
	// Note: Radarr add requires specific fields. We need rootFolderPath and qualityProfileId.
	// For a real setup, these should come from Config. We'll use defaults for now.
	// We might need to fetch profiles and root folders from Radarr API first.
	// For MVP, we'll try an optimistic payload.
	addPayload := map[string]interface{}{
		"title":            selectedMovie.Title,
		"titleSlug":        fmt.Sprintf("%d", selectedMovie.ID), // simplification
		"tmdbId":           selectedMovie.ID,
		"year":             time.Now().Year(), // ideally parse from ReleaseDate
		"qualityProfileId": 1, 
		"monitored":        true,
		"rootFolderPath":   "/movies", // adjust to user's setup if needed later
		"addOptions": map[string]bool{
			"searchForMovie": true,
		},
	}

	fmt.Println("Adding to Radarr...")
	if err := rClient.AddMovie(addPayload); err != nil {
		return nil, fmt.Errorf("failed to add movie to radarr: %w", err)
	}

	fmt.Println("Successfully added movie to Radarr queue!")
	return selectedMovie, nil
}
