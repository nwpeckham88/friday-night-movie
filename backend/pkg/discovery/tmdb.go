package discovery

import (
	"encoding/json"
	"fmt"
	"net/http"
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
