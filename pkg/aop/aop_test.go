package aop

import (
	"errors"
	"strings"
	"testing"
)

type audited struct{ Action string }

var svc = &Method{Package: "springlab.internal.service", Type: "CatalogService", Name: "FindAll",
	Exported: true, Annotations: []any{audited{"LIST"}}}
var web = &Method{Package: "springlab.internal.web", Type: "CatalogController", Name: "FindAll", Exported: true}
var hidden = &Method{Package: "springlab.internal.service", Type: "CatalogService", Name: "sleep"}

func must(t *testing.T, expr string) *Pointcut {
	t.Helper()
	pc, err := compile(expr, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	return pc
}

func TestPointcutDesignators(t *testing.T) {
	cases := []struct {
		expr string
		m    *Method
		want bool
	}{
		{"within(springlab.internal.service..*)", svc, true},
		{"within(springlab.internal.service..*)", web, false},
		{"within(springlab..*)", web, true},               // .. = any number of packages
		{"within(springlab.*)", svc, false},               // * = exactly one segment
		{"execution(public * *(..))", hidden, false},      // unexported = not public
		{"execution(* *..*Service.Find*(..))", svc, true}, // declaring type + name pattern
		{"execution(* *..*Service.Update*(..))", svc, false},
		{"@annotation(springlab.internal.service.Other)", svc, false},
		{"@annotation(audited)", svc, false}, // needs a binding — see below
		{"within(springlab..*) && !within(springlab.internal.web..*)", svc, true},
		{"within(springlab.internal.web..*) || execution(* Find*(..))", svc, true},
	}
	for _, c := range cases {
		pc, err := compile(c.expr, nil, "")
		if err != nil {
			if c.expr == "@annotation(audited)" && strings.Contains(err.Error(), "formal unbound") {
				continue
			}
			t.Fatalf("%s: %v", c.expr, err)
		}
		if got := pc.Matches(c.m, "catalogService"); got != c.want {
			t.Errorf("%s on %s: got %v, want %v", c.expr, c.m.Name, got, c.want)
		}
	}
}

func TestNamedPointcutsAndBadReference(t *testing.T) {
	Library("test.Pointcuts",
		P("serviceLayer", "within(springlab.internal.service..*)"),
		P("publicMethod", "execution(public * *(..))"),
		P("serviceOperation", "serviceLayer() && publicMethod()"))
	pc := must(t, "test.Pointcuts.serviceOperation()")
	if !pc.Matches(svc, "") || pc.Matches(hidden, "") {
		t.Fatal("named pointcut did not combine correctly")
	}
	if _, err := compile("serviceOperation()", nil, ""); err == nil ||
		!strings.Contains(err.Error(), "can't find referenced pointcut") {
		t.Fatalf("short reference must fail outside the library, got %v", err)
	}
	if _, err := compile("call(* *(..))", nil, ""); err == nil {
		t.Fatal("call() is not supported by Spring AOP")
	}
}

// TestOrderOfAdvice checks the fixed order inside one aspect and the
// nesting of aspects by @Order, including the error path.
func TestOrderOfAdvice(t *testing.T) {
	var log []string
	rec := func(s string) { log = append(log, s) }
	all := "within(springlab..*)"
	outer := []*Advice{
		{Kind: KindAround, around: func(p *ProceedingJoinPoint) (any, error) {
			rec("A:around-in")
			r, err := p.Proceed()
			rec("A:around-out")
			return r, err
		}},
	}
	inner := []*Advice{
		{Kind: KindAfter, after: func(*JoinPoint) { rec("B:after") }},
		{Kind: KindBefore, before: func(*JoinPoint) { rec("B:before") }},
		{Kind: KindAfterReturning, afterReturning: func(*JoinPoint, any) { rec("B:returning") }},
		{Kind: KindAfterThrowing, afterThrowing: func(*JoinPoint, error) { rec("B:throwing") }},
		{Kind: KindAround, around: func(p *ProceedingJoinPoint) (any, error) {
			rec("B:around-in")
			r, err := p.Proceed()
			rec("B:around-out")
			return r, err
		}},
	}
	for _, a := range append(outer, inner...) {
		a.Expr = all
	}
	h := &Handler{chains: map[string][]aspectAdvices{
		"FindAll": {{"A", outer}, {"B", inner}},
	}}

	_, _ = h.Invoke(svc, nil, func([]any) (any, error) { rec("method"); return "ok", nil })
	want := "A:around-in B:around-in B:before method B:returning B:after B:around-out A:around-out"
	if got := strings.Join(log, " "); got != want {
		t.Fatalf("success path:\n got %s\nwant %s", got, want)
	}

	log = nil
	boom := errors.New("boom")
	_, err := h.Invoke(svc, nil, func([]any) (any, error) { return nil, boom })
	if !errors.Is(err, boom) {
		t.Fatal("the error must not be suppressed")
	}
	if got := strings.Join(log, " "); !strings.Contains(got, "B:throwing B:after") ||
		strings.Contains(got, "returning") {
		t.Fatalf("error path: %s", got)
	}
}
