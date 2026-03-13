//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"google.golang.org/genai"
)

func main() {
	// Use a separate testing API key to avoid consuming the real API key's quota during tests.
	apiKey := os.Getenv("GEMINI_TEST_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_TEST_KEY not set. Please set the environment variable and try again.")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		log.Fatal(err)
	}

	model := "gemini-3.1-flash-lite-preview"
	prompt := "What is the newest Gemini model?"

	config := &genai.GenerateContentConfig{
		Temperature: genai.Ptr(float32(0.7)),
		Tools: []*genai.Tool{
			{
				GoogleSearch: &genai.GoogleSearch{},
			},
		},
	}

	fmt.Printf("Querying model %s...\n", model)

	response, err := client.Models.GenerateContent(ctx, model, genai.Text(prompt), config)
	if err != nil {
		log.Fatalf("gemini generation error: %v", err)
	}

	if len(response.Candidates) > 0 && len(response.Candidates[0].Content.Parts) > 0 {
		if text := response.Candidates[0].Content.Parts[0].Text; text != "" {
			fmt.Printf("\nResponse:\n%s\n", text)
		} else {
			fmt.Println("No text part found in response")
		}
	} else {
		fmt.Println("No candidates returned from the API")
	}
}
