package notify

import (
	"log/slog"
	"strings"

	"springlab/pkg/ioc"
)

// Individual assignment, variant 1: bean "upper", @Order(3), returns the
// message in upper case.
func init() {
	ioc.Component(newUpperNotifier, ioc.Name("upper"), ioc.Order(3))
}

type upperNotifier struct{}

func newUpperNotifier() *upperNotifier { return &upperNotifier{} }

// PostConstruct — @PostConstruct: logs the initialisation of the bean.
func (*upperNotifier) PostConstruct() {
	slog.Info("UPPER >> upperNotifier initialised (@PostConstruct)")
}

func (*upperNotifier) Send(message string) string {
	return "upper: " + strings.ToUpper(message)
}
func (*upperNotifier) Channel() string { return "upper" }
