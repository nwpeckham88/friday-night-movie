package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/user/friday-night-movie/pkg/discovery"
)

// Notifier is the interface for different notification providers
type Notifier interface {
	Notify(message string, movie *discovery.TMDBMovie) error
}

// LogNotifier is a stub that simply logs to the console
type LogNotifier struct{}

func (n *LogNotifier) Notify(message string, movie *discovery.TMDBMovie) error {
	if movie != nil {
		log.Printf("[Notification-Stub] %s: %s (%s)", message, movie.Title, movie.ReleaseDate)
	} else {
		log.Printf("[Notification-Stub] %s", message)
	}
	return nil
}

// DiscordNotifier sends notifications to a Discord Webhook
type DiscordNotifier struct {
	WebhookURL string
}

func (n *DiscordNotifier) Notify(message string, movie *discovery.TMDBMovie) error {
	if n.WebhookURL == "" {
		return fmt.Errorf("discord webhook url not configured")
	}

	payload := map[string]interface{}{
		"content": message,
	}

	if movie != nil {
		embed := map[string]interface{}{
			"title":       movie.Title,
			"description": movie.Overview,
			"color":       3447003, // Blue
			"fields": []map[string]interface{}{
				{
					"name":   "Rating",
					"value":  fmt.Sprintf("%.1f/10", movie.VoteAverage),
					"inline": true,
				},
				{
					"name":   "Release Date",
					"value":  movie.ReleaseDate,
					"inline": true,
				},
			},
		}

		if movie.PosterPath != "" {
			embed["thumbnail"] = map[string]string{
				"url": "https://image.tmdb.org/t/p/w500" + movie.PosterPath,
			}
		}

		payload["embeds"] = []interface{}{embed}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(n.WebhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord api error: %s", resp.Status)
	}

	return nil
}

// Global Notifier instance
var Instance Notifier = &LogNotifier{}

// SetNotifier allows swapping the notifier at runtime
func SetNotifier(n Notifier) {
	Instance = n
}
