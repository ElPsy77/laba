// Package autoconfigure holds the "auto-configurations" of the Go edition.
// Like classes listed in META-INF/spring/...AutoConfiguration.imports,
// each is applied only if its conditions hold, and they are processed
// AFTER the application's own beans so that @ConditionalOnMissingBean
// gives way to a bean you declared yourself.
package autoconfigure

import (
	"log/slog"
	"time"

	"springlab/pkg/env"
	"springlab/pkg/ioc"
)

func init() {
	// Actuator endpoints appear only when they are exposed in the settings.
	ioc.AutoConfiguration("WebEndpointAutoConfiguration", NewActuatorController,
		ioc.OnProperty("management.endpoints.web.exposure.include", "", false))

	// No spring.datasource.url in our settings -> a negative match.
	ioc.AutoConfiguration("DataSourceAutoConfiguration", NewDataSource,
		ioc.OnProperty("spring.datasource.url", "", false))

	// Depends on the profile: app.mail.enabled is false in dev, true in prod.
	ioc.AutoConfiguration("MailSenderAutoConfiguration", NewMailSender,
		ioc.OnProperty("app.mail.enabled", "true", false))

	// A default clock — unless the application declares its own Clock.
	ioc.AutoConfiguration("ClockAutoConfiguration", NewSystemClock,
		ioc.OnMissingBean[Clock]())
}

// DataSource stands in for a JDBC DataSource.
type DataSource struct{ URL string }

func NewDataSource(e *env.Environment) *DataSource {
	return &DataSource{URL: e.Get("spring.datasource.url", "")}
}

// MailSender stands in for JavaMailSender.
type MailSender struct{ From string }

func NewMailSender(e *env.Environment) *MailSender {
	slog.Debug("MailSender created: mail will really be sent")
	return &MailSender{From: e.Get("app.mail.from", "")}
}

// Clock is a replaceable time source.
type Clock interface{ Now() time.Time }

type SystemClock struct{}

func NewSystemClock() *SystemClock  { return &SystemClock{} }
func (*SystemClock) Now() time.Time { return time.Now() }
