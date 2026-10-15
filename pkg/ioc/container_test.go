package ioc

import (
	"reflect"
	"strings"
	"testing"
)

type greeter interface{ Hi() string }
type gA struct{}
type gB struct{}
type gC struct{}

func (gA) Hi() string { return "a" }
func (gB) Hi() string { return "b" }
func (gC) Hi() string { return "c" }

// fresh runs fn with an empty registry so tests do not see app beans.
func fresh(t *testing.T, fn func()) *Container {
	t.Helper()
	saved := registry
	registry = nil
	defer func() { registry = saved }()
	fn()
	c, err := New(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPrimaryFallbackQualifierAndCollections(t *testing.T) {
	type holder struct {
		def, q greeter
		all    []greeter
		byName map[string]greeter
	}
	c := fresh(t, func() {
		Component(func() gA { return gA{} }, Name("a"), Order(2))
		Component(func() gB { return gB{} }, Name("b"), Primary(), Order(1))
		Component(func() gC { return gC{} }, Name("c"), Fallback())
		Component(func(d, q greeter, all []greeter, m map[string]greeter) *holder {
			return &holder{d, q, all, m}
		}, Name("holder"), Qualifier(1, "c"))
	})
	if err := c.Refresh(); err != nil {
		t.Fatal(err)
	}
	h := MustGet[*holder](c)
	if h.def.Hi() != "b" || h.q.Hi() != "c" || len(h.byName) != 3 {
		t.Fatalf("unexpected injection: %+v", h)
	}
	order := ""
	for _, g := range h.all {
		order += g.Hi()
	}
	if order != "bac" {
		t.Fatalf("List order = %q, want bac (by @Order)", order)
	}
}

func TestNoPrimaryGivesNoUniqueBeanDefinition(t *testing.T) {
	c := fresh(t, func() {
		Component(func() gA { return gA{} }, Name("a"))
		Component(func() gB { return gB{} }, Name("b"))
		Component(func() gC { return gC{} }, Name("c"), Fallback())
	})
	_, err := c.resolve(reflect.TypeFor[greeter](), "")
	if err == nil || !strings.Contains(err.Error(), "expected single matching bean but found 3: a,b,c") {
		t.Fatalf("got %v", err)
	}
}

func TestFallbackLeavesSingleRegularCandidate(t *testing.T) {
	c := fresh(t, func() {
		Component(func() gA { return gA{} }, Name("a"))
		Component(func() gC { return gC{} }, Name("c"), Fallback())
	})
	if MustGet[greeter](c).Hi() != "a" {
		t.Fatal("@Fallback bean must give way to the regular one")
	}
}

type ticket struct{ n int }

var counter int

func TestPrototypeAndProvider(t *testing.T) {
	type office struct {
		direct *ticket
		p      *Provider[*ticket]
	}
	c := fresh(t, func() {
		Component(func() *ticket { counter++; return &ticket{counter} }, ScopePrototype())
		Component(func(d *ticket, p *Provider[*ticket]) *office { return &office{d, p} })
	})
	if err := c.Refresh(); err != nil {
		t.Fatal(err)
	}
	o := MustGet[*office](c)
	if o.p.GetObject() == o.p.GetObject() {
		t.Fatal("provider must return a new prototype instance each time")
	}
	if MustGet[*office](c) != o {
		t.Fatal("singleton must be the same instance")
	}
}

type cycA struct{}
type cycB struct{}

func TestCircularDependencyIsDetected(t *testing.T) {
	c := fresh(t, func() {
		Component(func(*cycB) *cycA { return &cycA{} })
		Component(func(*cycA) *cycB { return &cycB{} })
	})
	err := c.Refresh()
	if err == nil || !strings.Contains(err.Error(), "BeanCurrentlyInCreationException") {
		t.Fatalf("got %v", err)
	}
}
