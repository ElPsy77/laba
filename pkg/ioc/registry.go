// Package ioc is a small inversion-of-control container modelled on the
// Spring ApplicationContext: bean definitions, constructor injection,
// @Primary / @Fallback / @Qualifier, List and Map injection, @Order,
// singleton and prototype scopes, ObjectProvider and lifecycle callbacks.
package ioc

import (
	"fmt"
	"math"
	"reflect"
	"runtime"
	"strings"
	"unicode"
)

// Scope of a bean.
type Scope string

const (
	Singleton Scope = "singleton"
	Prototype Scope = "prototype"
)

// LowestPrecedence is the default @Order value: such beans go last.
const LowestPrecedence = math.MaxInt32

// Definition is the BeanDefinition: not the object but the recipe for it.
type Definition struct {
	Name       string
	Type       reflect.Type // the type the factory returns
	Factory    reflect.Value
	Scope      Scope
	Primary    bool
	Fallback   bool
	Order      int
	Qualifiers map[int]string // constructor argument index -> bean name
	Source     string         // where it was declared: component / @Bean method
	seq        int
}

// Option configures a Definition — the Go stand-in for annotations.
type Option func(*Definition)

func Name(n string) Option   { return func(d *Definition) { d.Name = n } }
func Primary() Option        { return func(d *Definition) { d.Primary = true } }
func Fallback() Option       { return func(d *Definition) { d.Fallback = true } }
func Order(n int) Option     { return func(d *Definition) { d.Order = n } }
func ScopePrototype() Option { return func(d *Definition) { d.Scope = Prototype } }
func Qualifier(arg int, bean string) Option {
	return func(d *Definition) { d.Qualifiers[arg] = bean }
}

// registry is filled by init() functions of the packages that the
// application imports: importing a package is the "component scan".
var registry []*Definition

// Component registers a constructor as a bean — @Component / @Service.
// The default name is the type name with a lower-case first letter.
func Component(constructor any, opts ...Option) {
	d := newDefinition(constructor, "component", opts)
	if d.Name == "" {
		d.Name = decapitalize(baseTypeName(d.Type))
	}
	registry = append(registry, d)
}

// Bean registers a factory function as a bean — a @Bean method inside a
// @Configuration class. The default name is the function name.
func Bean(factory any, opts ...Option) {
	d := newDefinition(factory, "@Bean method", opts)
	if d.Name == "" {
		d.Name = decapitalize(funcName(factory))
	}
	registry = append(registry, d)
}

func newDefinition(fn any, source string, opts []Option) *Definition {
	v := reflect.ValueOf(fn)
	t := v.Type()
	if t.Kind() != reflect.Func || t.NumOut() < 1 || t.NumOut() > 2 ||
		(t.NumOut() == 2 && t.Out(1) != errorType) {
		panic(fmt.Sprintf("ioc: %v is not a factory func(...) T or func(...) (T, error)", t))
	}
	d := &Definition{
		Type:       t.Out(0),
		Factory:    v,
		Scope:      Singleton,
		Order:      LowestPrecedence,
		Qualifiers: map[int]string{},
		Source:     source + " " + funcName(fn),
		seq:        len(registry),
	}
	for _, o := range opts {
		o(d)
	}
	return d
}

var errorType = reflect.TypeFor[error]()

func baseTypeName(t reflect.Type) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Name()
}

func funcName(fn any) string {
	full := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	full = full[strings.LastIndex(full, "/")+1:]
	full = strings.TrimSuffix(full, "-fm")
	return full[strings.LastIndex(full, ".")+1:]
}

func decapitalize(s string) string {
	s = strings.TrimPrefix(s, "New")
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// TypeName prints a type the way Spring prints a class name.
func TypeName(t reflect.Type) string { return t.String() }
