// Package lifecycle demonstrates the bean life cycle.
package lifecycle

import (
	"log/slog"
	"sync"
	"time"

	"springlab/internal/config"
	"springlab/pkg/ioc"
)

func init() {
	ioc.Component(NewLifecycleDemo)
}

// LifecycleDemo — Listing 7: records every stage of its own life cycle.
type LifecycleDemo struct {
	formatter *config.DateTimeFormatter
	mu        sync.Mutex
	events    []string
}

// NewLifecycleDemo — 1. the constructor. Only the dependencies passed as
// parameters (the formatter from task 4) are available here.
func NewLifecycleDemo(formatter *config.DateTimeFormatter) *LifecycleDemo {
	d := &LifecycleDemo{formatter: formatter}
	d.record("1. Constructor called")
	return d
}

// PostConstruct — 2. called after the object is fully assembled.
func (d *LifecycleDemo) PostConstruct() { d.record("2. @PostConstruct executed") }

// PreDestroy — 3. called when the context closes gracefully.
func (d *LifecycleDemo) PreDestroy() { d.record("3. @PreDestroy executed") }

func (d *LifecycleDemo) Events() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.events...)
}

func (d *LifecycleDemo) record(stage string) {
	line := d.formatter.Format(time.Now()) + "  " + stage
	d.mu.Lock()
	d.events = append(d.events, line)
	d.mu.Unlock()
	slog.Info("LIFECYCLE >> " + line)
}
