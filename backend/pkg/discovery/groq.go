package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type GroqClient struct {
	APIKey   string
	Endpoint string
	Model    string
	HTTP     *http.Client
}

func NewGroqClient(apiKey, endpoint, model string) (*GroqClient, error) {
	if endpoint == "" {
		endpoint = "https://api.groq.com/openai/v1/chat/completions"
	}
	if model == "" {
		model = "llama-3.3-70b-versatile"
	}
	return &GroqClient{
		APIKey:   apiKey,
		Endpoint: endpoint,
		Model:    model,
		HTTP:     &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (g *GroqClient) DiscoverMovie(userHistory []string, notify func(string)) (*GeminiResponse, error) {
	// Expert Persona Prompt (Unified with Gemini's logic)
	now := time.Now()
	dateStr := now.Format("January 02, 2006")
	historyContext := strings.Join(userHistory, ", ")

	prompt := fmt.Sprintf(`You are a World-class Movie Expert and Cinema Historian.
Your goal is to recommend ONE perfect, high-quality movie for the user.

Context:
- Today's Date: %s
- The user's recently watched/archived movies: %s

Instructions:
1. Act as an expert curator. Draw from your deep knowledge of film history, directorial styles, and cinematic movements.
2. Consider "deep cuts" and acclaimed cinema, not just blockbusters.
3. Suggest a movie that matches the "vibe" or "quality" of their history but offers something fresh.
4. DO NOT recommend items from the provided history list.
5. STRICTLY NO TV SHOWS/SERIES. ONLY FEATURE-LENGTH MOVIES.
6. Return ONLY JSON: {"title": "Movie", "year": 2024, "search_query": "Movie 2024"}
`, dateStr, historyContext)

	// Groq/OpenRouter Request (OpenAI Format)
	payload := map[string]interface{}{
		"model": g.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.7,
	}

	jsonPayload, _ := json.Marshal(payload)
	
	maxRetries := 2
	backoff := 2 * time.Second
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest("POST", g.Endpoint, bytes.NewBuffer(jsonPayload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+g.APIKey)

		resp, err := g.HTTP.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(backoff * time.Duration(1<<attempt))
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == 429 {
			if notify != nil {
				notify(fmt.Sprintf("Rate limited on Groq... waiting %v", backoff))
			}
			time.Sleep(backoff * time.Duration(1<<attempt))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("groq api error: %s", resp.Status)
		}

		var openAIResp struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
			return nil, err
		}

		if len(openAIResp.Choices) == 0 {
			return nil, fmt.Errorf("groq returned no choices")
		}

		textResponse := openAIResp.Choices[0].Message.Content

		// Clean up and parse JSON (Reuse common logic)
		startIdx := strings.Index(textResponse, "{")
		endIdx := strings.LastIndex(textResponse, "}")
		if startIdx == -1 || endIdx == -1 {
			return nil, fmt.Errorf("could not find JSON in response: %s", textResponse)
		}
		cleanResponse := textResponse[startIdx : endIdx+1]

		var result map[string]interface{}
		if err := json.Unmarshal([]byte(cleanResponse), &result); err != nil {
			return nil, err
		}

		log.Printf("Expert (via Groq) Suggested: %+v", result)
		return &GeminiResponse{
			Title:       result["title"].(string),
			Year:        int(result["year"].(float64)),
			SearchQuery: result["search_query"].(string),
		}, nil
	}

	return nil, fmt.Errorf("groq failed after retries: %w", lastErr)
}
