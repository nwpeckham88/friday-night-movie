package discovery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type TMDBClient struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func NewTMDBClient(apiKey string) *TMDBClient {
	return &TMDBClient{
		BaseURL: "https://api.themoviedb.org",
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

type TMDBMovie struct {
	ID               int      `json:"id"`
	Title            string   `json:"title"`
	Overview         string   `json:"overview"`
	ReleaseDate      string   `json:"release_date"`
	PosterPath       string   `json:"poster_path"`
	VoteAverage      float64  `json:"vote_average"`
	GenreIDs         []int    `json:"genre_ids"`
}

type TMDBResponse struct {
	Results []TMDBMovie `json:"results"`
}

// DiscoverMovies calls the TMDB discover API, you can pass custom query params
func (t *TMDBClient) DiscoverMovies(params string) ([]TMDBMovie, error) {
	url := fmt.Sprintf("%s/3/discover/movie?api_key=%s&%s", t.BaseURL, t.APIKey, params)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := t.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb api error: %s", resp.Status)
	}

	var result TMDBResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Results, nil
}
// SearchMovie resolves a literal string title into a TMDBMovie
func (t *TMDBClient) SearchMovie(query string) (*TMDBMovie, error) {
	movie, err := t.executeSearch(query)
	if err == nil && movie != nil {
		return movie, nil
	}

	// Fallback: If query contains a year at the end, try stripping it
	// LLMs often provide "Title (Year)" or "Title Year"
	parts := strings.Fields(query)
	if len(parts) > 1 {
		lastPart := parts[len(parts)-1]
		// Check if lastPart is a year (4 digits)
		if len(lastPart) == 4 || (len(lastPart) == 6 && strings.HasPrefix(lastPart, "(") && strings.HasSuffix(lastPart, ")")) {
			fallbackQuery := strings.Join(parts[:len(parts)-1], " ")
			fmt.Printf("TMDB: Search for '%s' failed, trying fallback: '%s'\n", query, fallbackQuery)
			return t.executeSearch(fallbackQuery)
		}
	}

	return nil, fmt.Errorf("no movie found on TMDB for query: %s", query)
}

func (t *TMDBClient) executeSearch(query string) (*TMDBMovie, error) {
	params := url.Values{}
	params.Add("api_key", t.APIKey)
	params.Add("query", query)
	params.Add("include_adult", "false")

	searchURL := fmt.Sprintf("%s/3/search/movie?%s", t.BaseURL, params.Encode())
	
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := t.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb api error: %s", resp.Status)
	}

	var result TMDBResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Results) == 0 {
		return nil, fmt.Errorf("no results")
	}

	return &result.Results[0], nil
}

// GetMovie fetches a single movie by its TMDB ID
func (t *TMDBClient) GetMovie(id int) (*TMDBMovie, error) {
	url := fmt.Sprintf("%s/3/movie/%d?api_key=%s", t.BaseURL, id, t.APIKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := t.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb api error: %s", resp.Status)
	}

	var result TMDBMovie
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
