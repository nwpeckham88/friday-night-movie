package notify

import (
	"log"

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

// Global Notifier instance
var Instance Notifier = &LogNotifier{}

// SetNotifier allows swapping the notifier at runtime
func SetNotifier(n Notifier) {
	Instance = n
}
