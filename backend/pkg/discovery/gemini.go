package discovery

import (
	"context"
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

// DiscoverMovie uses Gemini to think about the user's history and search for a great recommendation
func (g *GeminiClient) DiscoverMovie(userHistory []string) (string, error) {
	ctx := context.Background()

	// Use gemini-2.5-pro for thinking capabilities and grounded search
	model := "gemini-2.5-pro"

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
5. You MUST return ONLY the exact title of the movie and the year it was released in this format: "Title (Year)". Do not include quotes or any other text.
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
		return "", fmt.Errorf("gemini generation error: %w", err)
	}

	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini returned an empty response")
	}

	// Assuming the first part is the text response
	var textResponse string
	if text := response.Candidates[0].Content.Parts[0].Text; text != "" {
		textResponse = text
	} else {
		return "", fmt.Errorf("gemini return unexpected response type")
	}

	// Clean up the response just in case the LLM ignored the "only return title" rule
	cleanTitle := strings.TrimSpace(textResponse)
	cleanTitle = strings.ReplaceAll(cleanTitle, "\"", "")
	cleanTitle = strings.ReplaceAll(cleanTitle, "'", "")
	cleanTitle = strings.ReplaceAll(cleanTitle, "*", "")

	log.Printf("Gemini Suggested: %s", cleanTitle)
	return cleanTitle, nil
}
