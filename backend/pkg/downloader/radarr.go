package downloader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

// QueueEntry represents a download in progress
type QueueEntry struct {
	Title           string  `json:"title"`
	Size            float64 `json:"size"`
	Sizeleft        float64 `json:"sizeleft"`
	Status          string  `json:"status"`
	Timeleft        string  `json:"timeleft"`
	EstimatedCompletionTime string `json:"estimatedCompletionTime"`
	Id              int     `json:"id"`
	MovieId         int     `json:"movieId"`
}

type QueueResponse struct {
	Page          int          `json:"page"`
	PageSize      int          `json:"pageSize"`
	TotalRecords  int          `json:"totalRecords"`
	Records       []QueueEntry `json:"records"`
}

// Movie represents a movie in Radarr
type Movie struct {
	Title     string `json:"title"`
	TmdbId    int    `json:"tmdbId"`
	HasFile   bool   `json:"hasFile"`
	Monitored bool   `json:"monitored"`
}

func (c *Client) GetQueue() ([]QueueEntry, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v3/queue", c.BaseURL), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("radarr queue api error: %s", resp.Status)
	}

	var qRes QueueResponse
	if err := json.NewDecoder(resp.Body).Decode(&qRes); err != nil {
		return nil, err
	}

	return qRes.Records, nil
}

func (c *Client) GetMovies() ([]Movie, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v3/movie", c.BaseURL), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var movies []Movie
	if err := json.NewDecoder(resp.Body).Decode(&movies); err != nil {
		return nil, err
	}

	return movies, nil
}

// AddMovie adds a new movie to Radarr to download
// AddOptions requires Title, TmdbId, Year, RootFolderPath, QualityProfileId
func (c *Client) AddMovie(payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v3/movie", c.BaseURL), bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("radarr add movie error: %s", resp.Status)
	}

	return nil
}
