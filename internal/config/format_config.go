// Package config holds the configuration beans — the @Configuration classes.
package config

import (
	"strings"
	"time"

	"springlab/internal/app"
	"springlab/pkg/ioc"
)

// FormatConfig — @Configuration. A standard-library type cannot be marked
// as a component, so it is declared through a factory (@Bean method).
func init() {
	ioc.Bean(ReportDateFormatter)
}

// DateTimeFormatter plays the role of java.time.format.DateTimeFormatter:
// it accepts a Java-style pattern and formats time.Time with it.
type DateTimeFormatter struct {
	Pattern string
	layout  string
}

// ReportDateFormatter — Listing 6. The bean name is the function name:
// "reportDateFormatter". The pattern comes from
// @Value("${app.date-pattern:dd.MM.yyyy HH:mm:ss}").
func ReportDateFormatter(props app.Properties) *DateTimeFormatter {
	pattern := props.Get("app.date-pattern", "dd.MM.yyyy HH:mm:ss")
	return &DateTimeFormatter{Pattern: pattern, layout: javaToGoLayout(pattern)}
}

func (f *DateTimeFormatter) Format(t time.Time) string { return t.Format(f.layout) }

func javaToGoLayout(p string) string {
	return strings.NewReplacer(
		"yyyy", "2006", "MM", "01", "dd", "02",
		"HH", "15", "mm", "04", "ss", "05", "SSS", "000",
	).Replace(p)
}
