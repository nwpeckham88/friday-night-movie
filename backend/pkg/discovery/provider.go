package discovery

import (
	"fmt"
	"os"
	"strings"
)

// MovieDiscoverer is the interface for different LLM providers
type MovieDiscoverer interface {
	DiscoverMovie(history []string, tasteProfile string, rejectedMovies []string, notify func(string)) (*GeminiResponse, error)
	GenerateText(prompt string) (string, error)
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
