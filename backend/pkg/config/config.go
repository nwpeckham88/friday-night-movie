package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/user/friday-night-movie/pkg/db"
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
	PreferredLanguage string `json:"preferredLanguage"`
	StrictLanguage    bool   `json:"strictLanguage"`
	RadarrQualityProfileID int `json:"radarrQualityProfileId"`
	MinRating         float64 `json:"minRating"`
	DiscoveryMood     string  `json:"discoveryMood"`
	DiscoveryPersona  string  `json:"discoveryPersona"`
	DiscordWebhookURL string  `json:"discordWebhookUrl"`
	ExcludedEras      string  `json:"excludedEras"`  // e.g. "1980s, 1990s"
	ExcludedGenres    string  `json:"excludedGenres"` // e.g. "Horror, Documentary"
	SuggestInLibrary  bool    `json:"suggestInLibrary"`
	NoteToCurator     string  `json:"noteToCurator"`
}

type SpectrumDimension struct {
	Name      string  `json:"name"`
	PoleA     string  `json:"poleA"`
	PoleB     string  `json:"poleB"`
	StrengthA float64 `json:"strengthA"` // 0 to 10
	StrengthB float64 `json:"strengthB"` // 0 to 10
}

// AppState represents the active state of the app
type AppState struct {
	LastMovieTitle      string  `json:"lastMovieTitle"`
	LastMoviePosterPath string  `json:"lastMoviePosterPath"`
	LastMovieOverview   string  `json:"lastMovieOverview"`
	LastMovieRating     float64 `json:"lastMovieRating"`
	LastMovieID         int     `json:"lastMovieId"`
	LastMovieTrailerKey string  `json:"lastMovieTrailerKey,omitempty"`
	LastMovieReasoning   string  `json:"lastMovieReasoning"`
	LastMoviePathTheme   string  `json:"lastMoviePathTheme"`
	Status              string  `json:"status"`
	IsRunning           bool    `json:"isRunning"`
	IsSuggested         bool    `json:"isSuggested"`
	TasteProfile        string  `json:"tasteProfile"`
	RejectedMovies      []string `json:"rejectedMovies"`
	CinematicSpectrum   []SpectrumDimension `json:"cinematicSpectrum,omitempty"`
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
		"preferredLanguage": cfg.PreferredLanguage,
		"strictLanguage":    cfg.StrictLanguage,
		"radarrQualityProfileId": cfg.RadarrQualityProfileID,
		"minRating":         cfg.MinRating,
		
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

	if cfg.PreferredLanguage == "" && os.Getenv("PREFERRED_LANGUAGE") != "" {
		res["preferredLanguage"] = os.Getenv("PREFERRED_LANGUAGE")
	} else if cfg.PreferredLanguage == "" {
		res["preferredLanguage"] = "en"
	}

	if cfg.RadarrQualityProfileID == 0 {
		res["radarrQualityProfileId"] = 1 // Default to 1
	}

	if cfg.MinRating == 0 {
		res["minRating"] = 6.5 // Default to 6.5
	}

	if cfg.DiscoveryMood == "" {
		res["discoveryMood"] = "Balanced"
	} else {
		res["discoveryMood"] = cfg.DiscoveryMood
	}

	if cfg.DiscoveryPersona == "" {
		res["discoveryPersona"] = "The Movie Expert"
	} else {
		res["discoveryPersona"] = cfg.DiscoveryPersona
	}

	res["discordWebhookUrl"] = cfg.DiscordWebhookURL
	res["excludedEras"] = cfg.ExcludedEras
	res["excludedGenres"] = cfg.ExcludedGenres
	res["noteToCurator"] = cfg.NoteToCurator

	return res
}

func GetState() AppState {
	mutex.RLock()
	defer mutex.RUnlock()
	return memData.State
}

// Load reads from DB and performs migration if needed
func Load() error {
	mutex.Lock()
	defer mutex.Unlock()

	// 1. Try to load from DB
	configJson, _ := db.GetSetting("config")
	stateJson, _ := db.GetStateValue("state")

	if configJson != "" && stateJson != "" {
		_ = json.Unmarshal([]byte(configJson), &memData.Config)
		_ = json.Unmarshal([]byte(stateJson), &memData.State)
		fmt.Println("Loaded configuration and state from database.")
		return nil
	}

	// 2. Migration: Check if data.json exists
	if _, err := os.Stat(dataFile); err == nil {
		fmt.Println("Found legacy data.json, migrating to database...")
		b, err := os.ReadFile(dataFile)
		if err == nil {
			if err := json.Unmarshal(b, &memData); err == nil {
				// Save to DB
				configBytes, _ := json.Marshal(memData.Config)
				stateBytes, _ := json.Marshal(memData.State)
				db.SaveSetting("config", string(configBytes))
				db.SaveStateValue("state", string(stateBytes))
				fmt.Println("Migration successful.")
				
				// Optional: Rename legacy file to avoid re-migration
				os.Rename(dataFile, dataFile+".bak")
				return nil
			}
		}
	}

	return nil
}

func SaveConfig(cfg AppConfig) error {
	mutex.Lock()
	defer mutex.Unlock()
	memData.Config = cfg
	
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return db.SaveSetting("config", string(b))
}

func SaveState(state AppState) error {
	mutex.Lock()
	defer mutex.Unlock()
	memData.State = state
	
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return db.SaveStateValue("state", string(b))
}

// Deprecated: writeToFile is no longer used as we use DB now.
func writeToFile() error {
	return nil
}
