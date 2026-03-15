package discovery

import (
	"fmt"
	"os"
	"strings"
)

// ExpertSuggestion represents a movie suggested by the LLM
type ExpertSuggestion struct {
	Title       string `json:"title"`
	Year        int    `json:"year"`
	SearchQuery string `json:"search_query"`
	Reasoning   string `json:"reasoning"`
	PathTheme   string `json:"path_theme"`
}

// MovieDiscoverer is the interface for different LLM providers
type MovieDiscoverer interface {
	DiscoverMovie(history []string, tasteProfile string, rejectedMovies []string, failedSuggestions []string, weeklyContext string, notify func(string)) ([]ExpertSuggestion, error)
	GenerateText(prompt string) (string, error)
}

// GetWeeklyCinemaContext researches significant cinematic events for the current week
func GetWeeklyCinemaContext(p MovieDiscoverer, date string) (string, error) {
	prompt := fmt.Sprintf("Research and summarize major cinematic anniversaries (movies, directors, actors), notable deaths, or significant film festivals/events happening during the week of %s. Focus on historical significance and 'archival' interest. Provide a concise bulleted list of the top 5 most interesting events.", date)
	return p.GenerateText(prompt)
}

// GetProvider returns the appropriate discovery provider based on configuration
func GetProvider(uiKey string, provider string) (MovieDiscoverer, error) {
	if provider == "" {
		provider = os.Getenv("LLM_PROVIDER")
		if provider == "" {
			provider = "gemini" // Default
		}
	}

	// Helper to mask keys in logs
	mask := func(s string) string {
		if len(s) < 8 {
			return "****"
		}
		return s[:4] + "..." + s[len(s)-4:]
	}

	switch provider {
	case "gemini":
		key := uiKey
		keySource := "UI (Settings)"
		if envKey := os.Getenv("GEMINI_KEY"); envKey != "" {
			key = envKey
			keySource = "ENV (GEMINI_KEY)"
		}
		os.Stdout.WriteString(fmt.Sprintf("[Provider] Using Gemini (Key source: %s, Mask: %s)\n", keySource, mask(key)))
		return NewGeminiClient(key)

	case "groq":
		key := os.Getenv("GROQ_API_KEY")
		keySource := "ENV (GROQ_API_KEY)"
		if key == "" {
			// As a last resort, check if the UI key looks like a Groq key (usually starts with gsk_)
			if strings.HasPrefix(uiKey, "gsk_") {
				key = uiKey
				keySource = "UI (Settings - GSK detected)"
			}
		}
		
		endpoint := os.Getenv("LLM_ENDPOINT")
		if endpoint == "" {
			endpoint = "https://api.groq.com/openai/v1/chat/completions"
		}
		model := os.Getenv("LLM_MODEL")
		if model == "" {
			model = "llama-3.3-70b-versatile"
		}
		
		if key == "" {
			return nil, fmt.Errorf("groq api key not found (checked GROQ_API_KEY and UI settings)")
		}
		
		os.Stdout.WriteString(fmt.Sprintf("[Provider] Using Groq (Model: %s, Key source: %s, Mask: %s)\n", model, keySource, mask(key)))
		return NewGroqClient(key, endpoint, model)

	case "openrouter":
		key := os.Getenv("OPENROUTER_API_KEY")
		keySource := "ENV (OPENROUTER_API_KEY)"
		if key == "" {
			// OpenRouter keys often start with sk-or-
			if strings.HasPrefix(uiKey, "sk-or-") {
				key = uiKey
				keySource = "UI (Settings - OR detected)"
			}
		}
		
		endpoint := os.Getenv("LLM_ENDPOINT")
		if endpoint == "" {
			endpoint = "https://openrouter.ai/api/v1/chat/completions"
		}
		model := os.Getenv("LLM_MODEL")
		if model == "" {
			model = "meta-llama/llama-3.1-70b-instruct"
		}
		
		if key == "" {
			return nil, fmt.Errorf("openrouter api key not found (checked OPENROUTER_API_KEY and UI settings)")
		}
		
		os.Stdout.WriteString(fmt.Sprintf("[Provider] Using OpenRouter (Model: %s, Key source: %s, Mask: %s)\n", model, keySource, mask(key)))
		return NewGroqClient(key, endpoint, model)

	default:
		return NewGeminiClient(uiKey)
	}
}
