// Package aop is Spring AOP for Go: aspects, AspectJ-style pointcut
// expressions, the five advice types, ordering of aspects and proxies.
//
// Go has no runtime class generation (CGLIB) and no dynamic proxies, so
// proxies are generated at build time by cmd/aopgen (`go generate`). At
// run time an AutoProxyCreator — a BeanPostProcessor of the container —
// wraps a bean in its proxy if at least one pointcut matches one of its
// methods. Everything else follows Spring: only method executions of
// container beans are join points, and a call through the receiver inside
// the bean (self-invocation) bypasses the proxy.
package aop

import (
	"fmt"
	"reflect"
)

// Method describes a method that can be a join point. The generator emits
// one per interface method of a proxied type.
type Method struct {
	Package     string // dotted Go package path: springlab.internal.service
	Type        string // declaring type: CatalogService
	Name        string // FindAll
	Exported    bool   // Go's "public"
	Annotations []any  // annotation values written as // @Audited(...) in the source
}

// QualifiedType is the fully qualified declaring type used by within().
func (m *Method) QualifiedType() string { return m.Package + "." + m.Type }

// Annotation returns the annotation of type A on the method, if present.
func Annotation[A any](m *Method) (A, bool) {
	for _, a := range m.Annotations {
		if v, ok := a.(A); ok {
			return v, true
		}
	}
	var zero A
	return zero, false
}

func (m *Method) annotationOfType(t reflect.Type) (any, bool) {
	for _, a := range m.Annotations {
		if reflect.TypeOf(a) == t {
			return a, true
		}
	}
	return nil, false
}

// Signature mirrors org.aspectj.lang.Signature.
type Signature struct{ m *Method }

func (s Signature) Name() string { return s.m.Name }

// ToShortString prints CatalogService.FindAll(..) — as in Spring.
func (s Signature) ToShortString() string { return fmt.Sprintf("%s.%s(..)", s.m.Type, s.m.Name) }

func (s Signature) DeclaringTypeName() string { return s.m.QualifiedType() }

// JoinPoint — the method execution being intercepted.
type JoinPoint struct {
	Method *Method
	target any // the real object
	this   any // the proxy
	args   []any
}

func (jp *JoinPoint) Signature() Signature { return Signature{jp.Method} }
func (jp *JoinPoint) Args() []any          { return append([]any(nil), jp.args...) }
func (jp *JoinPoint) Target() any          { return jp.target }
func (jp *JoinPoint) This() any            { return jp.this }

// ProceedingJoinPoint is given to @Around advice.
type ProceedingJoinPoint struct {
	*JoinPoint
	next func(args []any) (any, error)
}

// Proceed runs the next advice or, at the end of the chain, the target
// method. Without a call to Proceed the method never runs.
func (p *ProceedingJoinPoint) Proceed() (any, error) { return p.next(p.args) }

// ProceedWith runs the call with substituted arguments: proceed(Object[]).
func (p *ProceedingJoinPoint) ProceedWith(args []any) (any, error) { return p.next(args) }

// Arg returns args[i] as T (the zero value for nil) — used by generated proxies.
func Arg[T any](args []any, i int) T {
	if v, ok := args[i].(T); ok {
		return v
	}
	var zero T
	return zero
}
