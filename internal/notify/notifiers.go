package notify

import (
	"log/slog"

	"springlab/pkg/ioc"
)

// Listing 3. Three implementations. In Java the bean name, @Primary,
// @Fallback and @Order are annotations on the class; in Go they are
// options passed when the package registers its components. These init()
// calls run because main imports the package — that is the "component scan".
func init() {
	ioc.Component(NewConsoleNotifier, ioc.Name("console"), ioc.Order(1))
	ioc.Component(newEmailNotifier, ioc.Name("email"), ioc.Primary(), ioc.Order(2)) // chosen by default
	ioc.Component(newNoopNotifier, ioc.Name("noop"), ioc.Fallback(), ioc.Order(99)) // fallback option
}

// ConsoleNotifier — @Component("console") @Order(1).
type ConsoleNotifier struct{}

func NewConsoleNotifier() *ConsoleNotifier { return &ConsoleNotifier{} }

func (*ConsoleNotifier) Send(message string) string {
	slog.Info("CONSOLE >> " + message)
	return "console: " + message
}
func (*ConsoleNotifier) Channel() string { return "console" }

// emailNotifier — @Component("email") @Primary @Order(2), package-private.
type emailNotifier struct{}

func newEmailNotifier() *emailNotifier { return &emailNotifier{} }

func (*emailNotifier) Send(message string) string {
	slog.Info("EMAIL >> " + message)
	return "email: " + message
}
func (*emailNotifier) Channel() string { return "email" }

// noopNotifier — @Component("noop") @Fallback @Order(99).
type noopNotifier struct{}

func newNoopNotifier() *noopNotifier { return &noopNotifier{} }

func (*noopNotifier) Send(string) string { return "noop" }
func (*noopNotifier) Channel() string    { return "noop" }
