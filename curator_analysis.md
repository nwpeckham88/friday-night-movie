# Curator Engine Analysis

The current engine has evolved from a simple recommender to a sophisticated discovery agent. Here is an analysis of its current state and suitability as a "Cinematic Curator."

## What Works
- **Thematic Continuity ("Paths")**: The multi-phase LLM prompts (Reel Analysis → Path Selection → Curator's Notes) successfully force the agent to think in terms of cinematic movements and trajectories rather than just similarity.
- **Contextual Grounding**: The addition of "Weekly Cinema Context" provides a real-world anchor (anniversaries, news) that makes suggestions feel "alive" and timely.
- **Deduplication Hygiene**: The synchronized local library and suggestion ledger ensure the agent doesn't repeat itself, maintaining the "archival discovery" feel.
- **Visual Transparency**: The "Thinking UI" adds the necessary friction and depth to the user experience, making the discovery feel like a deliberate process.

## What's Missing / Doesn't Work
- **Lack of "Global Steering"**: The user currently has no way to provide permanent, high-level directives (e.g., "No Musicals," "Focus on 70s Japanese Cinema") beyond the excluded eras/genres.
- **Limited Interaction with History**: We pivot to the "Discovery Ledger" for taste, but the ledger is currently just a list of titles. We aren't extracting deeper "whys" from past rejections vs. acceptances beyond the text taste profile.
- **Linear Curation**: Every session is a "fresh start" with a path. There's no concept of a "Curated Season" or long-term cinematic journey that spans multiple Friday nights.

## Proposed Strategy: "The Curator's Journal"
To elevate FNM to a true curator, we should implement:

1.  **"Note to Curator" (Global Steering)**: A persistent text field in settings passed to the LLM as the "User's Manifesto" or "House Rules."
2.  **Trajectory Persistence**: The LLM should be able to see not just the titles in the ledger, but the *Previous Path Themes*. This allows it to decide whether to *continue* the current trajectory or *pivot* deliberately.
3.  **Deeper Rejection Logic**: When a user rejects a movie, we should ask (or the LLM should infer) *why*. "Too violent" is different from "Already seen."

---

### Immediate Next Steps
1.  **Implement "Note to Curator"**: Add this to [AppConfig](file:///home/dietpi/Development/friday-night-movie/backend/pkg/config/config.go#13-32), DB, and UI.
2.  **Inject Path History**: Pass the last 5 `path_theme` values from the ledger into the [DiscoverMovie](file:///home/dietpi/Development/friday-night-movie/backend/pkg/discovery/groq.go#37-188) prompt.
3.  **Refine "Suggest in Library"**: Ensure the re-watch logic explicitly justifies *why* this specific owned movie is being "re-curated" (e.g., "It's the 50th anniversary of this specific cut").
