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
	Title  string `json:"title"`
	Year   int    `json:"year"`
	TMDBID int    `json:"tmdb_id"`
}

// DiscoverMovie uses Gemini to think about the user's history and search for a great recommendation
func (g *GeminiClient) DiscoverMovie(userHistory []string) (*GeminiResponse, error) {
	ctx := context.Background()

	// Use gemini-2.5-flash for capabilities and grounded search
	model := "gemini-2.5-flash"

	// Current Context
	now := time.Now()
	dateStr := now.Format("January 02, 2006")

	historyContext := "None"
	if len(userHistory) > 0 {
		historyContext = strings.Join(userHistory, ", ")
	}

	prompt := fmt.Sprintf(`You are a top-tier movie recommendation engine.
Your task is to recommend ONE perfect movie for the user to watch tonight.

Context:
- Today's Date: %s
- The user's recently watched/archived movies: %s

Instructions:
1. Think deeply about the user's taste based on their history.
2. Consider the current date/season.
3. Use Google Search to find highly-rated trending movies or hidden gems that match this profile.
4. DO NOT recommend a movie they have already watched.
5. You MUST return ONLY a JSON object containing the movie's title, release year, and TMDB ID. Do not include markdown formatting or any other text.
Format: {"title": "Movie Title", "year": 2024, "tmdb_id": 123456}
`, dateStr, historyContext)

	log.Printf("Prompting Gemini: \n%s\n", prompt)

	// Configure Generation Config with Search Grounding
	config := &genai.GenerateContentConfig{
		Temperature: genai.Ptr(float32(0.7)), // A bit of creativity
		Tools: []*genai.Tool{
			{
				GoogleSearch: &genai.GoogleSearch{}, // Enable Search Grounding
			},
		},
	}

	// For gemini-2.5-pro, Thinking is enabled by default or we can explicitly request it if needed.
	// The Thinking feature allows the model to reason before answering.

	response, err := g.Client.Models.GenerateContent(ctx, model, genai.Text(prompt), config)
	if err != nil {
		return nil, fmt.Errorf("gemini generation error: %w", err)
	}

	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini returned an empty response")
	}

	// Assuming the first part is the text response
	var textResponse string
	if text := response.Candidates[0].Content.Parts[0].Text; text != "" {
		textResponse = text
	} else {
		return nil, fmt.Errorf("gemini return unexpected response type")
	}

	// Clean up the response just in case the LLM returned markdown
	cleanResponse := strings.TrimSpace(textResponse)
	cleanResponse = strings.TrimPrefix(cleanResponse, "```json")
	cleanResponse = strings.TrimPrefix(cleanResponse, "```")
	cleanResponse = strings.TrimSuffix(cleanResponse, "```")
	cleanResponse = strings.TrimSpace(cleanResponse)

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(cleanResponse), &result); err != nil {
		return nil, fmt.Errorf("failed to parse gemini json response: %w - response was: %s", err, cleanResponse)
	}

	log.Printf("Gemini Suggested: %+v", result)
	return &GeminiResponse{
		Title:  result["title"].(string),
		Year:   int(result["year"].(float64)),
		TMDBID: int(result["tmdb_id"].(float64)),
	}, nil
}
