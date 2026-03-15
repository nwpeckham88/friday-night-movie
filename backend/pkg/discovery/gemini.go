package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/user/friday-night-movie/pkg/config"
	"google.golang.org/genai"
)

type GeminiClient struct {
	Client *genai.Client
}

func NewGeminiClient(apiKey string) (*GeminiClient, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return nil, err
	}
	return &GeminiClient{
		Client: client,
	}, nil
}

// DiscoverMovie uses Gemini to think about the user's history and search for a great recommendation
func (g *GeminiClient) DiscoverMovie(userHistory []string, spectrum []config.SpectrumDimension, rejectedMovies []string, failedSuggestions []string, weeklyContext string, pathHistory []string, globalNote string, userRequest string, notify func(string)) ([]ExpertSuggestion, error) {
	ctx := context.Background()

	models := []string{"gemini-3-flash-preview", "gemini-3.1-flash-lite-preview", "gemini-2.0-flash"}

	// Current Context
	now := time.Now()
	dateStr := now.Format("January 02, 2006")

	historyContext := "None"
	if len(userHistory) > 0 {
		historyContext = strings.Join(userHistory, ", ")
	}

	rejectedContext := "None"
	if len(rejectedMovies) > 0 {
		rejectedContext = strings.Join(rejectedMovies, ", ")
	}

	failedContext := "None"
	if len(failedSuggestions) > 0 {
		failedContext = strings.Join(failedSuggestions, ", ")
	}

	spectrumContext, _ := json.Marshal(spectrum)
	if len(spectrum) == 0 {
		spectrumContext = []byte("No spectrum mapping yet. Perform broad discovery.")
	}

	cfg := config.GetConfig()
	mood := cfg.DiscoveryMood
	if mood == "" { mood = "Balanced" }
	persona := cfg.DiscoveryPersona
	if persona == "" { persona = "The Movie Expert" }
	excludedEras := cfg.ExcludedEras
	excludedGenres := cfg.ExcludedGenres

	prompt := fmt.Sprintf(`You are %s.
Your goal is to perform a deep-dive "Cinematic Discovery" session.

CURATION PROCESS:
1. PHASE 1: REEL ANALYSIS - Examine the user's history, Cinematic Spectrum, and Previous Path Themes. Identify a thematic "thread" based on their spectral weights.
2. PHASE 2: PATH SELECTION - Choose a specific thematic PATH for this session (e.g., "Neon Noir & Nightmares").
3. PHASE 3: SELECTION - Suggest 5 movies that fit this PATH.
4. PHASE 4: CURATOR'S NOTES - For each movie, provide a DEEP CINEMATIC ANALYSIS (3-4 paragraphs).
   - Paragraph 1: Cinematic Significance & Technique.
   - Paragraph 2: Behind-the-scenes trivia or historical context.
   - Paragraph 3: Specific personal relevance (e.g., "Since you favor Surrealism (9/10), you'll appreciate the dream-logic here...").

Context:
- Today's Date: %s
- Cinematic Spectrum (Dimensions & Weights): %s
- Recent History: %s
- Rejected: %s
- FAILED SUGGESTIONS (STRICTLY DO NOT RECOMMEND): %s
- EXCLUDED ERAS (STRICTLY DO NOT RECOMMEND): %s
- EXCLUDED GENRES (STRICTLY DO NOT RECOMMEND): %s
- PREVIOUS PATH THEMES: %s
- USER'S MANIFESTO: %s
- SPECIFIC USER REQUEST (PRIORITIZE THIS): %s

Instructions:
1. Act according to your persona (%s).
2. Respect the mood: %s.
3. If a SPECIFIC USER REQUEST is provided, ensure your suggestions strictly follow that request while maintaining your personal style and spectral nuance.
4. Return ONLY a JSON list of objects: [{"title": "Movie", "year": 2024, "search_query": "Movie 2024", "reasoning": "...", "path_theme": "PATH NAME HERE"}]
`, persona, dateStr, string(spectrumContext), historyContext, rejectedContext, failedContext, excludedEras, excludedGenres, strings.Join(pathHistory, ", "), globalNote, userRequest, persona, mood)

	// Configure Generation Config with Search Grounding
	genConfig := &genai.GenerateContentConfig{
		Temperature: genai.Ptr(float32(0.7)),
		Tools: []*genai.Tool{
			{
				GoogleSearch: &genai.GoogleSearch{},
			},
		},
	}

	var lastErr error
	for _, model := range models {
		maxRetries := 2
		backoff := 2 * time.Second

		for attempt := 0; attempt <= maxRetries; attempt++ {
			log.Printf("Prompting Gemini (Model: %s, Attempt: %d): \n%s\n", model, attempt+1, prompt)

			response, err := g.Client.Models.GenerateContent(ctx, model, genai.Text(prompt), genConfig)
			if err != nil {
				lastErr = err
				// Check if it's a rate limit error (429)
				if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "RESOURCE_EXHAUSTED") {
					if attempt < maxRetries {
						waitTime := backoff * time.Duration(1<<attempt)
						if notify != nil {
							notify(fmt.Sprintf("Rate limited on %s... waiting %v", model, waitTime))
						}
						log.Printf("Rate limited on %s, waiting %v...", model, waitTime)
						time.Sleep(waitTime)
						continue
					}
					// If max retries reached for this model, fallback to next model
					log.Printf("Max retries reached for %s, falling back...", model)
					break 
				}
				// For other errors, we might want to fallback immediately or return
				log.Printf("Gemini error on model %s: %v", model, err)
				break
			}

			if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
				lastErr = fmt.Errorf("gemini returned an empty response")
				break
			}

			// Assuming the first part is the text response
			var textResponse string
			if text := response.Candidates[0].Content.Parts[0].Text; text != "" {
				textResponse = text
			} else {
				lastErr = fmt.Errorf("gemini return unexpected response type")
				break
			}

			// Clean up the response just in case the LLM returned extra text
			startIdx := strings.Index(textResponse, "[")
			endIdx := strings.LastIndex(textResponse, "]")
			if startIdx == -1 || endIdx == -1 {
				// Try object fallback
				startIdx = strings.Index(textResponse, "{")
				endIdx = strings.LastIndex(textResponse, "}")
				if startIdx == -1 || endIdx == -1 {
					lastErr = fmt.Errorf("could not find JSON in gemini response: %s", textResponse)
					break
				}
				cleanResponse := textResponse[startIdx : endIdx+1]
				var single ExpertSuggestion
				if err := json.Unmarshal([]byte(cleanResponse), &single); err == nil {
					return []ExpertSuggestion{single}, nil
				}
				lastErr = fmt.Errorf("failed to parse gemini json object response: %s", cleanResponse)
				break
			}
			cleanResponse := textResponse[startIdx : endIdx+1]

			var suggestions []ExpertSuggestion
			if err := json.Unmarshal([]byte(cleanResponse), &suggestions); err != nil {
				lastErr = fmt.Errorf("failed to parse gemini json list response: %w - response was: %s", err, cleanResponse)
				break
			}

			log.Printf("Gemini Suggested (via %s): %d movies", model, len(suggestions))
			return suggestions, nil
		}
	}

	return nil, fmt.Errorf("all gemini models failed or rate limited. Last error: %w", lastErr)
}

func (g *GeminiClient) GenerateText(prompt string) (string, error) {
	ctx := context.Background()
	model := "gemini-2.0-flash" // Use a fast model for text generation

	response, err := g.Client.Models.GenerateContent(ctx, model, genai.Text(prompt), nil)
	if err != nil {
		return "", err
	}

	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini returned an empty response")
	}

	return response.Candidates[0].Content.Parts[0].Text, nil
}
