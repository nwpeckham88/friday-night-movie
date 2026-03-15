package discovery

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/user/friday-night-movie/pkg/config"
)

// UpdateTasteProfile takes existing data and returns a new interpretation of user taste via the Cinematic Spectrum
func UpdateTasteProfile(provider MovieDiscoverer, currentSpectrum []config.SpectrumDimension, history []string, rejected []string, latestAction string) ([]config.SpectrumDimension, string, error) {
	historyContext := "None"
	if len(history) > 0 {
		historyContext = strings.Join(history, ", ")
	}

	rejectedContext := "None"
	if len(rejected) > 0 {
		rejectedContext = strings.Join(rejected, ", ")
	}

	spectrumJSON, _ := json.Marshal(currentSpectrum)

	prompt := fmt.Sprintf(`You are a World-class Film Critic and Cinematic Taste Analyst.
Your task is to update the user's "Cinematic Spectrum" based on a new interaction.

The Cinematic Spectrum consists of 16 dimensions, each with two poles (A and B).
For each dimension, you must assign a STRENGTH (0-10) to Pole A and a STRENGTH (0-10) to Pole B.
This allows a user to enjoy both poles (e.g., StrengthA=8, StrengthB=8 means they love both Grounded and Surreal films).

CURRENT SPECTRUM:
%s

USER CONTEXT:
- History: %s
- Rejected: %s
- LATEST INTERACTION: %s

INSTRUCTIONS:
1. Analyze the interaction. If they loved a movie, increase the strengths of the poles that movie represents. If they rejected it, increase the strength of the OPPOSING poles or decrease the current strengths.
2. Return a JSON object with TWO fields:
   - "spectrum": The full list of 16 dimensions with updated strengths.
   - "summary": A 2-paragraph subjective "interpretation" of their current state (e.g., "The user is currently in a 'Neon-Noir' phase...").

DIMENSIONS TO MAINTAIN:
1. Pacing (Staccato vs. Legato)
2. Visual Texture (Naturalism vs. Expressionism)
3. Narrative Logic (Linear vs. Labyrinthine)
4. Thematic Density (Literal vs. Subtextual)
5. Emotional Temperature (Cynical vs. Earnest)
6. Dialogue Style (Logorrheic vs. Laconic)
7. Scope (Micro vs. Macro)
8. Atmosphere (Ominous vs. Comforting)
9. Realism (Grounded vs. Surreal)
10. Character Agency (Active vs. Passive)
11. Genre Relationship (Orthodox vs. Deconstructive)
12. Sound Scape (Diegetic vs. Operatic)
13. Intellection (Intellectual vs. Visceral)
14. Era Anchoring (Archive vs. Contemporary)
15. Cultural Lens (Domestic vs. World)
16. Transgression (Safe vs. Provocative)

Return ONLY JSON.
`, string(spectrumJSON), historyContext, rejectedContext, latestAction)

	resp, err := provider.GenerateText(prompt)
	if err != nil {
		return nil, "", err
	}

	// Simple JSON extraction
	start := strings.Index(resp, "{")
	end := strings.LastIndex(resp, "}")
	if start == -1 || end == -1 {
		return nil, "", fmt.Errorf("failed to find JSON in LLM response")
	}

	var update struct {
		Spectrum []config.SpectrumDimension `json:"spectrum"`
		Summary  string              `json:"summary"`
	}
	if err := json.Unmarshal([]byte(resp[start:end+1]), &update); err != nil {
		return nil, "", err
	}

	return update.Spectrum, update.Summary, nil
}
