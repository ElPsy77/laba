package service

import (
	"strings"
	"testing"

	"springlab/internal/aspect" // the aspects of the application
	"springlab/pkg/aop"
	"springlab/pkg/ioc"
)

func context(t *testing.T) *ioc.Container {
	t.Helper()
	c, err := ioc.New(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Refresh(); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestBeanIsAProxyOfTheInterface(t *testing.T) {
	c := context(t)
	bean := ioc.MustGet[CatalogService](c)
	if !aop.IsAopProxy(bean) {
		t.Fatalf("expected a proxy, got %T", bean)
	}
	if _, ok := aop.TargetOf(bean).(*catalogService); !ok {
		t.Fatal("the proxy must hold the real catalogService")
	}
	// Like a JDK dynamic proxy: the proxy implements the interface but is
	// not the concrete type, so asking for the concrete type fails.
	_, err := ioc.Get[*catalogService](c)
	if err == nil || !strings.Contains(err.Error(), "BeanNotOfRequiredTypeException") {
		t.Fatalf("got %v", err)
	}
}

func TestSelfInvocationGoesThroughProxyAfterFix(t *testing.T) {
	c := context(t)
	counter := ioc.MustGet[*aspect.CallCounterAspect](c)
	if _, err := ioc.MustGet[CatalogService](c).RemoveTwice(5); err != nil {
		t.Fatal(err)
	}
	calls := map[string]int64{}
	for _, s := range counter.Stats() {
		calls[s.Method] = s.Calls
	}
	if calls["CatalogService.RemoveTwice(..)"] != 1 || calls["CatalogService.Remove(..)"] != 2 {
		t.Fatalf("with the self reference both inner calls must be advised: %v", calls)
	}
}

func TestPlainObjectIsNotAdvised(t *testing.T) {
	// An object created with "new" is not a container bean: no proxy, no advice.
	s := newCatalogService()
	if got, _ := s.RemoveTwice(1); got != "Removed item no. 1; Removed item no. 2" {
		t.Fatal(got)
	}
	if aop.IsAopProxy(s) {
		t.Fatal("must not be a proxy")
	}
}
