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
	GeminiKey    string `json:"geminiKey"`
	LLMProvider  string `json:"llmProvider"`
}

// AppState represents the active state of the app
type AppState struct {
	LastMovieTitle      string  `json:"lastMovieTitle"`
	LastMoviePosterPath string  `json:"lastMoviePosterPath"`
	LastMovieOverview   string  `json:"lastMovieOverview"`
	LastMovieRating     float64 `json:"lastMovieRating"`
	LastMovieID         int     `json:"lastMovieId"`
	Status              string  `json:"status"`
	IsRunning           bool    `json:"isRunning"`
	IsSuggested         bool    `json:"isSuggested"`
	TasteProfile        string  `json:"tasteProfile"`
	RejectedMovies      []string `json:"rejectedMovies"`
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
	
	cfg := memData.Config
	
	// Fallback to Env variables if JSON value is empty
	if cfg.JellyfinURL == "" {
		cfg.JellyfinURL = os.Getenv("JELLYFIN_URL")
	}
	if cfg.JellyfinKey == "" {
		cfg.JellyfinKey = os.Getenv("JELLYFIN_KEY")
	}
	if cfg.RadarrURL == "" {
		cfg.RadarrURL = os.Getenv("RADARR_URL")
	}
	if cfg.RadarrKey == "" {
		cfg.RadarrKey = os.Getenv("RADARR_KEY")
	}
	if cfg.TMDBKey == "" {
		cfg.TMDBKey = os.Getenv("TMDB_KEY")
	}
	
	// For testing purposes, prioritize testing API key
	if testKey := os.Getenv("GEMINI_TEST_KEY"); testKey != "" {
		cfg.GeminiKey = testKey
	} else if cfg.GeminiKey == "" {
		cfg.GeminiKey = os.Getenv("GEMINI_KEY")
	}

	if cfg.LLMProvider == "" {
		cfg.LLMProvider = os.Getenv("LLM_PROVIDER")
		if cfg.LLMProvider == "" {
			cfg.LLMProvider = "gemini"
		}
	}

	return cfg
}

// GetFrontendConfig returns the config state and indicates if a value was sourced from the .env file
func GetFrontendConfig() map[string]interface{} {
	mutex.RLock()
	defer mutex.RUnlock()
	
	cfg := memData.Config
	
	res := map[string]interface{}{
		"jellyfinUrl": cfg.JellyfinURL,
		"jellyfinKey": cfg.JellyfinKey,
		"radarrUrl":   cfg.RadarrURL,
		"radarrKey":   cfg.RadarrKey,
		"tmdbKey":     cfg.TMDBKey,
		"geminiKey":   cfg.GeminiKey,
		"llmProvider": cfg.LLMProvider,
		
		"jellyfinUrlFromEnv": false,
		"jellyfinKeyFromEnv": false,
		"radarrUrlFromEnv":   false,
		"radarrKeyFromEnv":   false,
		"tmdbKeyFromEnv":     false,
		"geminiKeyFromEnv":   false,
	}

	if cfg.JellyfinURL == "" && os.Getenv("JELLYFIN_URL") != "" {
		res["jellyfinUrl"] = os.Getenv("JELLYFIN_URL")
		res["jellyfinUrlFromEnv"] = true
	}
	if cfg.JellyfinKey == "" && os.Getenv("JELLYFIN_KEY") != "" {
		res["jellyfinKey"] = os.Getenv("JELLYFIN_KEY")
		res["jellyfinKeyFromEnv"] = true
	}
	if cfg.RadarrURL == "" && os.Getenv("RADARR_URL") != "" {
		res["radarrUrl"] = os.Getenv("RADARR_URL")
		res["radarrUrlFromEnv"] = true
	}
	if cfg.RadarrKey == "" && os.Getenv("RADARR_KEY") != "" {
		res["radarrKey"] = os.Getenv("RADARR_KEY")
		res["radarrKeyFromEnv"] = true
	}
	if cfg.TMDBKey == "" && os.Getenv("TMDB_KEY") != "" {
		res["tmdbKey"] = os.Getenv("TMDB_KEY")
		res["tmdbKeyFromEnv"] = true
	}
	
	if os.Getenv("GEMINI_TEST_KEY") != "" {
		res["geminiKey"] = os.Getenv("GEMINI_TEST_KEY")
		res["geminiKeyFromEnv"] = true
	} else if cfg.GeminiKey == "" && os.Getenv("GEMINI_KEY") != "" {
		res["geminiKey"] = os.Getenv("GEMINI_KEY")
		res["geminiKeyFromEnv"] = true
	}

	if cfg.LLMProvider == "" && os.Getenv("LLM_PROVIDER") != "" {
		res["llmProvider"] = os.Getenv("LLM_PROVIDER")
		res["llmProviderFromEnv"] = true
	} else if cfg.LLMProvider == "" {
		res["llmProvider"] = "gemini"
	}

	return res
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
