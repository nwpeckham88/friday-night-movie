package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/user/friday-night-movie/pkg/config"
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

func (g *GroqClient) DiscoverMovie(userHistory []string, spectrum []config.SpectrumDimension, rejectedMovies []string, failedSuggestions []string, weeklyContext string, pathHistory []string, globalNote string, userRequest string, notify func(string)) ([]ExpertSuggestion, error) {
	// Expert Persona Prompt (Unified with Gemini's logic)
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
		startIdx := strings.Index(textResponse, "[")
		endIdx := strings.LastIndex(textResponse, "]")
		if startIdx == -1 || endIdx == -1 {
			// Try object fallback
			startIdx = strings.Index(textResponse, "{")
			endIdx = strings.LastIndex(textResponse, "}")
			if startIdx == -1 || endIdx == -1 {
				return nil, fmt.Errorf("could not find JSON in response: %s", textResponse)
			}
			cleanResponse := textResponse[startIdx : endIdx+1]
			var single ExpertSuggestion
			if err := json.Unmarshal([]byte(cleanResponse), &single); err == nil {
				return []ExpertSuggestion{single}, nil
			}
			return nil, fmt.Errorf("failed to parse expert JSON object response: %s", cleanResponse)
		}
		cleanResponse := textResponse[startIdx : endIdx+1]

		var suggestions []ExpertSuggestion
		if err := json.Unmarshal([]byte(cleanResponse), &suggestions); err != nil {
			return nil, err
		}

		log.Printf("Expert (via Groq) Suggested: %d movies", len(suggestions))
		return suggestions, nil
	}

	return nil, fmt.Errorf("groq failed after retries: %w", lastErr)
}

func (g *GroqClient) GenerateText(prompt string) (string, error) {
	// Groq/OpenRouter Request (OpenAI Format)
	payload := map[string]interface{}{
		"model": g.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.5,
	}

	jsonPayload, _ := json.Marshal(payload)
	
	req, err := http.NewRequest("POST", g.Endpoint, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.APIKey)

	resp, err := g.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("groq api error: %s", resp.Status)
	}

	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return "", err
	}

	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("groq returned no choices")
	}

	return openAIResp.Choices[0].Message.Content, nil
}
