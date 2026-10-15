package ioc

import (
	"fmt"
	"reflect"
	"strings"
)

// PropertyResolver is what conditions need from the Environment.
type PropertyResolver interface {
	Property(key string) (string, bool)
	ActiveProfiles() []string
}

// ConditionContext is passed to every condition.
type ConditionContext struct {
	Env       PropertyResolver
	container *Container
}

// HasBeanOfType tells whether a bean of type t is already registered.
func (c ConditionContext) HasBeanOfType(t reflect.Type) bool {
	return len(c.container.candidates(t)) > 0
}

// Condition decides whether a bean definition is registered.
type Condition interface {
	Name() string
	Matches(ctx ConditionContext) (bool, string)
}

// ConditionOutcome is one line of the conditions report.
type ConditionOutcome struct {
	Bean      string `json:"bean"`
	Source    string `json:"source"`
	Condition string `json:"condition"`
	Message   string `json:"message"`
}

// Report is the auto-configuration (conditions) report: what was applied
// and why, what was rejected and which condition failed.
type Report struct {
	Positive []ConditionOutcome `json:"positiveMatches"`
	Negative []ConditionOutcome `json:"negativeMatches"`
}

// ---- @Profile ---------------------------------------------------------------

type profileCondition struct{ expr string }

// Profile is @Profile("dev"), @Profile("!prod"), @Profile("!dev & !prod"),
// @Profile("test | prod"). With no active profile, "default" is active.
func Profile(expr string) Option { return When(profileCondition{expr}) }

func (p profileCondition) Name() string { return fmt.Sprintf("@Profile(%q)", p.expr) }

func (p profileCondition) Matches(ctx ConditionContext) (bool, string) {
	active := ctx.Env.ActiveProfiles()
	if len(active) == 0 {
		active = []string{"default"}
	}
	is := func(name string) bool {
		for _, a := range active {
			if a == name {
				return true
			}
		}
		return false
	}
	result := false
	for _, alt := range strings.Split(p.expr, "|") { // OR of ANDs
		all := true
		for _, term := range strings.Split(alt, "&") {
			term = strings.TrimSpace(term)
			if strings.HasPrefix(term, "!") {
				all = all && !is(strings.TrimSpace(term[1:]))
			} else {
				all = all && is(term)
			}
		}
		result = result || all
	}
	return result, fmt.Sprintf("active profiles %v", active)
}

// ---- @ConditionalOnProperty ------------------------------------------------

type propertyCondition struct {
	name, having   string
	matchIfMissing bool
}

// OnProperty is @ConditionalOnProperty(name, havingValue, matchIfMissing).
// An empty havingValue means "set and not equal to false".
func OnProperty(name, havingValue string, matchIfMissing bool) Option {
	return When(propertyCondition{name, havingValue, matchIfMissing})
}

func (p propertyCondition) Name() string {
	return fmt.Sprintf("@ConditionalOnProperty(%s=%s)", p.name, p.having)
}

func (p propertyCondition) Matches(ctx ConditionContext) (bool, string) {
	v, ok := ctx.Env.Property(p.name)
	if !ok {
		return p.matchIfMissing, fmt.Sprintf("did not find property '%s'", p.name)
	}
	if p.having == "" {
		return !strings.EqualFold(v, "false"), fmt.Sprintf("found property '%s' = %s", p.name, v)
	}
	if strings.EqualFold(v, p.having) {
		return true, fmt.Sprintf("found property '%s' having value '%s'", p.name, v)
	}
	return false, fmt.Sprintf("found different value in property '%s' (%s)", p.name, v)
}

// ---- @ConditionalOnMissingBean -------------------------------------------------

type missingBeanCondition struct{ t reflect.Type }

// OnMissingBean is @ConditionalOnMissingBean: the bean is created only if
// the application has not declared its own bean of that type.
func OnMissingBean[T any]() Option { return When(missingBeanCondition{reflect.TypeFor[T]()}) }

func (m missingBeanCondition) Name() string { return "@ConditionalOnMissingBean" }

func (m missingBeanCondition) Matches(ctx ConditionContext) (bool, string) {
	if ctx.HasBeanOfType(m.t) {
		return false, fmt.Sprintf("found beans of type '%s'", m.t)
	}
	return true, fmt.Sprintf("did not find any beans of type '%s'", m.t)
}

// evaluate runs the conditions of d and records the outcome.
func (c *Container) evaluate(d *Definition) bool {
	ctx := ConditionContext{Env: c.env, container: c}
	for _, cond := range d.Conditions {
		ok, msg := cond.Matches(ctx)
		o := ConditionOutcome{Bean: d.Name, Source: d.Source, Condition: cond.Name(), Message: msg}
		if !ok {
			c.report.Negative = append(c.report.Negative, o)
			return false
		}
		c.report.Positive = append(c.report.Positive, o)
	}
	return true
}

// ConditionsReport — what /actuator/conditions and --debug print.
func (c *Container) ConditionsReport() Report { return c.report }
