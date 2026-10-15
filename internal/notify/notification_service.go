package notify

import (
	"sort"

	"springlab/pkg/ioc"
)

// Listing 4. A service with four injection points. Parameter 1 carries
// @Qualifier("console"): in Go the qualifier is attached to the argument
// index when the component is registered.
func init() {
	ioc.Component(NewNotificationService, ioc.Qualifier(1, "console"))
}

type NotificationService struct {
	primary Notifier            // @Primary -> emailNotifier
	console Notifier            // stated explicitly
	all     []Notifier          // all of them, ordered by @Order
	byName  map[string]Notifier // key is the bean name
}

// NewNotificationService — constructor injection: every field is set once
// and the object is always fully assembled.
func NewNotificationService(primary Notifier, console Notifier,
	all []Notifier, byName map[string]Notifier) *NotificationService {
	return &NotificationService{primary: primary, console: console, all: all, byName: byName}
}

func (s *NotificationService) ViaPrimary(message string) string { return s.primary.Send(message) }

func (s *NotificationService) ViaConsole(message string) string { return s.console.Send(message) }

func (s *NotificationService) ViaAll(message string) []string {
	out := make([]string, 0, len(s.all))
	for _, n := range s.all {
		out = append(out, n.Send(message))
	}
	return out
}

func (s *NotificationService) Names() []string {
	out := make([]string, 0, len(s.byName))
	for n := range s.byName {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
