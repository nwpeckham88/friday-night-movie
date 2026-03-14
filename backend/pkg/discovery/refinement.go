package discovery

import (
	"fmt"
	"strings"
)

// UpdateTasteProfile takes existing data and returns a new 1000-word interpretation of user taste
func UpdateTasteProfile(provider MovieDiscoverer, oldProfile string, history []string, rejected []string, latestAction string) (string, error) {
	historyContext := "None"
	if len(history) > 0 {
		historyContext = strings.Join(history, ", ")
	}

	rejectedContext := "None"
	if len(rejected) > 0 {
		rejectedContext = strings.Join(rejected, ", ")
	}

	prompt := fmt.Sprintf(`You are a World-class Film Critic and Cinematic Taste Analyst.
Your task is to update the user's "Cinematic Taste Profile" based on a new interaction.

Current Profile:
%s

User History (Watched/Archived):
%s

Recently Rejected/Not Interested:
%s

LATEST INTERACTION:
%s

INSTRUCTIONS:
1. Synthesize all information to create a deep, insightful interpretation of the user's movie taste.
2. DO NOT just list movies. Explain WHY they like certain things (e.g., "The user seems drawn to nihilistic neo-noirs but rejects them if the pacing is too deliberate").
3. Incorporate the LATEST INTERACTION prominently. If they rejected a movie, what does that say about the boundaries of their taste?
4. Keep the summary comprehensive but under 1000 words.
5. Maintain a sophisticated, expert tone.
6. Return ONLY the new profile text.
`, oldProfile, historyContext, rejectedContext, latestAction)

	return provider.GenerateText(prompt)
}
