package config

import (
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"springlab/internal/notify"
	"springlab/pkg/ioc"
)

// Listing 10. A report on the contents of the container, printed at
// start-up by a CommandLineRunner.
func init() {
	ioc.Component(NewContainerReport)
}

type ContainerReport struct{ context *ioc.Container }

func NewContainerReport(context *ioc.Container) *ContainerReport {
	return &ContainerReport{context: context}
}

func (r *ContainerReport) Run([]string) error {
	slog.Info(fmt.Sprintf("Bean definitions in total: %d", r.context.BeanDefinitionCount()))
	slog.Info(fmt.Sprintf("Notifier implementations: %v",
		r.context.BeanNamesForType(reflect.TypeFor[notify.Notifier]())))
	bean, err := r.context.GetBean("notificationService")
	if err != nil {
		return err
	}
	slog.Info(fmt.Sprintf("Type of the notificationService bean: %T", bean))
	for _, name := range r.context.BeanDefinitionNames() {
		d, _ := r.context.Definition(name)
		if strings.Contains(strings.ToLower(name), "notifier") || strings.HasPrefix(d.Type.String(), "*") {
			slog.Info(fmt.Sprintf("  bean: %-22s %s", name, d.Type))
		}
	}
	return nil
}
