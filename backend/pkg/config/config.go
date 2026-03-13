package config

import (
	"encoding/json"
	"os"
	"sync"
)

// AppConfig represents the user settings
type AppConfig struct {
	JellyfinURL  string `json:"jellyfinUrl"`
	JellyfinKey  string `json:"jellyfinKey"`
	RadarrURL    string `json:"radarrUrl"`
	RadarrKey    string `json:"radarrKey"`
	TMDBKey      string `json:"tmdbKey"`
}

// AppState represents the active state of the app
type AppState struct {
	LastMovieTitle      string `json:"lastMovieTitle"`
	LastMoviePosterPath string `json:"lastMoviePosterPath"`
	LastMovieOverview   string `json:"lastMovieOverview"`
	LastMovieRating     float64 `json:"lastMovieRating"`
}

// Data represents the saved JSON structure
type Data struct {
	Config AppConfig `json:"config"`
	State  AppState  `json:"state"`
}

var (
	memData Data
	mutex   sync.RWMutex
	dataDir string = "data"
	dataFile string = "data/data.json"
)

func init() {
	// Initialize default state
	memData = Data{}
}

func GetConfig() AppConfig {
	mutex.RLock()
	defer mutex.RUnlock()
	return memData.Config
}

func GetState() AppState {
	mutex.RLock()
	defer mutex.RUnlock()
	return memData.State
}

func SaveConfig(cfg AppConfig) error {
	mutex.Lock()
	defer mutex.Unlock()
	memData.Config = cfg
	return writeToFile()
}

func SaveState(state AppState) error {
	mutex.Lock()
	defer mutex.Unlock()
	memData.State = state
	return writeToFile()
}

// Load reads the JSON file into memory if it exists
func Load() error {
	mutex.Lock()
	defer mutex.Unlock()
	
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		os.MkdirAll(dataDir, 0755)
	}

	b, err := os.ReadFile(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // OK if it doesn't exist yet
		}
		return err
	}

	return json.Unmarshal(b, &memData)
}

func writeToFile() error {
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		os.MkdirAll(dataDir, 0755)
	}

	b, err := json.MarshalIndent(memData, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(dataFile, b, 0644)
}
