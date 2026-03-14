package discovery

import (
	"os"
)

// MovieDiscoverer is the interface for different LLM providers
type MovieDiscoverer interface {
	DiscoverMovie(history []string, notify func(string)) (*GeminiResponse, error)
}

// GetProvider returns the appropriate discovery provider based on configuration
func GetProvider(apiKey string, provider string) (MovieDiscoverer, error) {
	if provider == "" {
		provider = os.Getenv("LLM_PROVIDER")
		if provider == "" {
			provider = "gemini" // Default
		}
	}

	switch provider {
	case "gemini":
		return NewGeminiClient(apiKey)
	case "groq", "openrouter":
		// We'll use the same GroqClient for both if they are OpenAI compatible
		groqKey := os.Getenv("GROQ_API_KEY")
		if groqKey == "" {
			groqKey = apiKey // Fallback to provided key if intended
		}
		endpoint := os.Getenv("LLM_ENDPOINT")
		model := os.Getenv("LLM_MODEL")
		return NewGroqClient(groqKey, endpoint, model)
	default:
		return NewGeminiClient(apiKey)
	}
}
