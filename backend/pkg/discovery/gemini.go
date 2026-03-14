package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

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

type GeminiResponse struct {
	Title       string `json:"title"`
	Year        int    `json:"year"`
	SearchQuery string `json:"search_query"`
}

// DiscoverMovie uses Gemini to think about the user's history and search for a great recommendation
func (g *GeminiClient) DiscoverMovie(userHistory []string, tasteProfile string, rejectedMovies []string, failedSuggestions []string, notify func(string)) (*GeminiResponse, error) {
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

	if tasteProfile == "" {
		tasteProfile = "No profile established yet. Start with broad high-quality recommendations."
	}

	prompt := fmt.Sprintf(`You are a World-class Movie Expert and Cinema Historian.
Your goal is to recommend ONE perfect, high-quality movie for the user based on their taste.

Context:
- Today's Date: %s
- Your current interpretation of user's taste: %s
- The user's recently watched/archived movies: %s
- Movies the user has REJECTED/NOT INTERESTED (STRICTLY DO NOT RECOMMEND THESE): %s
- Movies you suggested IN THIS SESSION that were ALREADY IN LIBRARY or REJECTED (STRICTLY DO NOT RECOMMEND THESE): %s

Instructions:
1. Act as an expert curator. Draw from your deep knowledge of film history, directorial styles, and cinematic movements.
2. Consider "deep cuts" and acclaimed cinema, not just blockbusters.
3. Suggest a movie that matches the "vibe" or "quality" of their history and profile but offers something fresh.
4. DO NOT recommend items from the provided history list, rejected list, or failed suggestion list.
5. STRICTLY NO TV SHOWS/SERIES. ONLY FEATURE-LENGTH MOVIES.
6. Return ONLY JSON: {"title": "Movie", "year": 2024, "search_query": "Movie 2024"}
`, dateStr, tasteProfile, historyContext, rejectedContext, failedContext)

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
			startIdx := strings.Index(textResponse, "{")
			endIdx := strings.LastIndex(textResponse, "}")
			if startIdx == -1 || endIdx == -1 {
				lastErr = fmt.Errorf("could not find JSON object in gemini response: %s", textResponse)
				break
			}
			cleanResponse := textResponse[startIdx : endIdx+1]

			var result map[string]interface{}
			if err := json.Unmarshal([]byte(cleanResponse), &result); err != nil {
				lastErr = fmt.Errorf("failed to parse gemini json response: %w - response was: %s", err, cleanResponse)
				break
			}

			log.Printf("Gemini Suggested (via %s): %+v", model, result)
			return &GeminiResponse{
				Title:       result["title"].(string),
				Year:        int(result["year"].(float64)),
				SearchQuery: result["search_query"].(string),
			}, nil
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
