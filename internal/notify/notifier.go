// Package notify: the Notifier interface, its implementations and the service.
package notify

// Notifier — Listing 2.
type Notifier interface {
	Send(message string) string // returns what was "sent"
	Channel() string            // channel name for the report
}
