package logic

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/user/friday-night-movie/pkg/discovery"
	"github.com/user/friday-night-movie/pkg/downloader"
	"github.com/user/friday-night-movie/pkg/media"
)

func TestRunFridayNightRoutine(t *testing.T) {
	// Mock Jellyfin Server
	jServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := media.JellyfinItemsResponse{
			Items: []media.JellyfinItem{
				{Name: "Existing Movie 1"},
			},
		}
		json.NewEncoder(w).Encode(res)
	}))
	defer jServer.Close()

	// Mock Radarr Server
	rServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			res := []downloader.Movie{
				{Title: "Existing Movie 2"},
			}
			json.NewEncoder(w).Encode(res)
		} else if r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer rServer.Close()

	// Mock TMDB Server
	tServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := discovery.TMDBResponse{
			Results: []discovery.TMDBMovie{
				{ID: 1, Title: "Existing Movie 1", VoteAverage: 8.0}, // Should be filtered out by Jellyfin
				{ID: 2, Title: "Existing Movie 2", VoteAverage: 7.5}, // Should be filtered out by Radarr
				{ID: 3, Title: "New Awesome Movie", VoteAverage: 9.0}, // Should be selected
			},
		}
		json.NewEncoder(w).Encode(res)
	}))
	defer tServer.Close()

	// Initialize clients with test server URLs
	jClient := media.NewJellyfinClient(jServer.URL, "fake-key")
	
	// Create TMDB client and override BaseURL
	tClient := discovery.NewTMDBClient("fake-key")
	tClient.BaseURL = tServer.URL
	
	rClient := downloader.NewClient(rServer.URL, "fake-key")

	// Run logic
	movie, err := RunFridayNightRoutine(jClient, tClient, rClient)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if movie == nil {
		t.Fatalf("Expected to return a movie, got nil")
	}

	if movie.Title != "New Awesome Movie" {
		t.Errorf("Expected 'New Awesome Movie', git '%s'", movie.Title)
	}
}
