package media

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type JellyfinClient struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func NewJellyfinClient(baseURL, apiKey string) *JellyfinClient {
	return &JellyfinClient{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

type JellyfinItem struct {
	Name         string   `json:"Name"`
	Id           string   `json:"Id"`
	Type         string   `json:"Type"`
	ProviderIds  Provider `json:"ProviderIds"`
	Genres       []string `json:"Genres"`
	PlayAccess   string   `json:"PlayAccess"`
}

type Provider struct {
	Tmdb string `json:"Tmdb"`
	Imdb string `json:"Imdb"`
}

type JellyfinItemsResponse struct {
	Items            []JellyfinItem `json:"Items"`
	TotalRecordCount int            `json:"TotalRecordCount"`
}

// GetMovies returns all movies in the Jellyfin library to help build a profile of what not to download again
func (c *JellyfinClient) GetMovies(userId string) ([]JellyfinItem, error) {
	// If userId is empty, we can try to get all items generally, but jellyfin requires a UserId usually for "Items"
	// To make this simple, we can query /Items with Recursive=true and IncludeItemTypes=Movie
	url := fmt.Sprintf("%s/Items?Recursive=true&IncludeItemTypes=Movie", c.BaseURL)
	if userId != "" {
		url += fmt.Sprintf("&UserId=%s", userId)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("X-Emby-Token", c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jellyfin api error fetching items: %s", resp.Status)
	}

	var res JellyfinItemsResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return res.Items, nil
}
