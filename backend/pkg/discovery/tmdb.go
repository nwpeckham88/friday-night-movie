package discovery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
	OriginalLanguage string   `json:"original_language"`
	TrailerKey       string   `json:"trailer_key,omitempty"`
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
func (t *TMDBClient) SearchMovie(title string, year int) (*TMDBMovie, error) {
	params := url.Values{}
	params.Add("api_key", t.APIKey)
	params.Add("query", title)
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
		return nil, fmt.Errorf("no results found on TMDB for: %s", title)
	}

	// Iterate through top results to find the correct year
	// We check the first 5 results for a year match
	limit := 5
	if len(result.Results) < limit {
		limit = len(result.Results)
	}

	// 1. First, search for exactly the title
	for i := 0; i < limit; i++ {
		m := result.Results[i]
		if year > 0 && len(m.ReleaseDate) >= 4 {
			var rYear int
			fmt.Sscanf(m.ReleaseDate[:4], "%d", &rYear)
			if rYear == year {
				return &m, nil
			}
		} else if year == 0 {
			// If no year provided, take the first one
			return &m, nil
		}
	}

	// 2. If no year match, fallback to the first result as a best-effort
	fmt.Printf("TMDB: Could not find exact year match for '%s' (%d), falling back to first result: '%s' (%s)\n", title, year, result.Results[0].Title, result.Results[0].ReleaseDate)
	return &result.Results[0], nil
}

// GetTrendingMovies fetches the currently trending movies from TMDB
func (t *TMDBClient) GetTrendingMovies() ([]TMDBMovie, error) {
	url := fmt.Sprintf("%s/3/trending/movie/week?api_key=%s", t.BaseURL, t.APIKey)

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
func (t *TMDBClient) GetMovieTrailer(movieID int) (string, error) {
	u := fmt.Sprintf("%s/3/movie/%d/videos?api_key=%s", t.BaseURL, movieID, t.APIKey)
	
	resp, err := t.HTTP.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("tmdb api error: %s", resp.Status)
	}

	var result struct {
		Results []struct {
			Key  string `json:"key"`
			Site string `json:"site"`
			Type string `json:"type"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	for _, v := range result.Results {
		if v.Site == "YouTube" && (v.Type == "Trailer" || v.Type == "Teaser") {
			return v.Key, nil
		}
	}

	return "", nil
}
