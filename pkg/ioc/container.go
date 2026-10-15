package ioc

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"strings"
)

// PostConstructor is the counterpart of @PostConstruct.
type PostConstructor interface{ PostConstruct() }

// PreDestroyer is the counterpart of @PreDestroy.
type PreDestroyer interface{ PreDestroy() }

// CommandLineRunner runs once the context has been refreshed.
type CommandLineRunner interface{ Run(args []string) error }

// Container is the ApplicationContext.
type Container struct {
	defs       []*Definition
	byName     map[string]*Definition
	singletons map[string]reflect.Value
	inCreation map[string]bool
	created    []string // creation order, destroyed in reverse
	closed     bool
}

// New reads ALL bean definitions first (the registry of every imported
// package plus the given ready-made instances); objects are created later
// by Refresh. That is why declaration order does not matter.
func New(instances map[string]any) (*Container, error) {
	c := &Container{
		byName:     map[string]*Definition{},
		singletons: map[string]reflect.Value{},
		inCreation: map[string]bool{},
	}
	names := make([]string, 0, len(instances))
	for n := range instances {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		v := reflect.ValueOf(instances[n])
		d := &Definition{Name: n, Type: v.Type(), Scope: Singleton,
			Order: LowestPrecedence, Source: "registered singleton", seq: -1}
		c.singletons[n] = v
		if err := c.register(d); err != nil {
			return nil, err
		}
	}
	for _, d := range registry {
		if err := c.register(d); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (c *Container) register(d *Definition) error {
	if old, dup := c.byName[d.Name]; dup {
		return fmt.Errorf("BeanDefinitionOverrideException: invalid bean definition with name '%s' "+
			"defined in %s: there is already %s bound", d.Name, d.Source, old.Source)
	}
	c.byName[d.Name] = d
	c.defs = append(c.defs, d)
	return nil
}

// Refresh pre-instantiates every singleton, as Spring does at start-up:
// configuration errors surface now, not on first use.
func (c *Container) Refresh() error {
	for _, d := range c.defs {
		if d.Scope == Singleton {
			if _, err := c.instance(d); err != nil {
				return err
			}
		}
	}
	return nil
}

// Close destroys singletons in reverse creation order (@PreDestroy).
func (c *Container) Close() {
	if c.closed {
		return
	}
	c.closed = true
	for i := len(c.created) - 1; i >= 0; i-- {
		if pd, ok := c.singletons[c.created[i]].Interface().(PreDestroyer); ok {
			pd.PreDestroy()
		}
	}
}

// ---- talking to the container ----------------------------------------

func (c *Container) BeanDefinitionCount() int { return len(c.defs) }

func (c *Container) BeanDefinitionNames() []string {
	out := make([]string, len(c.defs))
	for i, d := range c.defs {
		out[i] = d.Name
	}
	return out
}

func (c *Container) Definition(name string) (*Definition, bool) {
	d, ok := c.byName[name]
	return d, ok
}

// BeanNamesForType — getBeanNamesForType(Notifier.class).
func (c *Container) BeanNamesForType(t reflect.Type) []string {
	var out []string
	for _, d := range c.candidates(t) {
		out = append(out, d.Name)
	}
	return out
}

// GetBean — context.getBean("name").
func (c *Container) GetBean(name string) (any, error) {
	d, ok := c.byName[name]
	if !ok {
		return nil, fmt.Errorf("NoSuchBeanDefinitionException: No bean named '%s' available", name)
	}
	v, err := c.instance(d)
	if err != nil {
		return nil, err
	}
	return v.Interface(), nil
}

// Get — context.getBean(T.class).
func Get[T any](c *Container) (T, error) {
	var zero T
	v, err := c.resolve(reflect.TypeFor[T](), "")
	if err != nil {
		return zero, err
	}
	return v.Interface().(T), nil
}

// MustGet panics on failure; convenient in main and tests.
func MustGet[T any](c *Container) T {
	v, err := Get[T](c)
	if err != nil {
		panic(err)
	}
	return v
}

// BeansOfType returns every bean of type T ordered by @Order.
func BeansOfType[T any](c *Container) ([]T, error) {
	v, err := c.resolve(reflect.TypeFor[[]T](), "")
	if err != nil {
		return nil, err
	}
	return v.Interface().([]T), nil
}

// ---- creation and dependency resolution ------------------------------

func (c *Container) instance(d *Definition) (reflect.Value, error) {
	if d.Scope == Singleton {
		if v, ok := c.singletons[d.Name]; ok {
			return v, nil
		}
	}
	if c.inCreation[d.Name] {
		return reflect.Value{}, fmt.Errorf("BeanCurrentlyInCreationException: Error creating bean "+
			"with name '%s': Requested bean is currently in creation: "+
			"Is there an unresolvable circular reference?", d.Name)
	}
	c.inCreation[d.Name] = true
	defer delete(c.inCreation, d.Name)

	if d.Scope == Singleton {
		slog.Debug(fmt.Sprintf("Creating shared instance of singleton bean '%s'", d.Name))
	}

	ft := d.Factory.Type()
	args := make([]reflect.Value, ft.NumIn())
	for i := range args {
		a, err := c.resolve(ft.In(i), d.Qualifiers[i])
		if err != nil {
			return reflect.Value{}, fmt.Errorf("UnsatisfiedDependencyException: Error creating bean "+
				"with name '%s' (%s): Unsatisfied dependency expressed through constructor "+
				"parameter %d: %w", d.Name, d.Source, i, err)
		}
		args[i] = a
	}
	out := d.Factory.Call(args) // 1. constructor, 2. dependencies passed in
	if len(out) == 2 && !out[1].IsNil() {
		return reflect.Value{}, fmt.Errorf("BeanCreationException: Error creating bean with name "+
			"'%s': %w", d.Name, out[1].Interface().(error))
	}
	bean := out[0]
	if pc, ok := bean.Interface().(PostConstructor); ok {
		pc.PostConstruct() // 3. @PostConstruct
	}
	if d.Scope == Singleton {
		c.singletons[d.Name] = bean
		c.created = append(c.created, d.Name)
	}
	return bean, nil
}

// binder is implemented by *Provider[T]: the container hands itself over.
type binder interface{ bind(*Container) }

var containerType = reflect.TypeFor[*Container]()

func (c *Container) resolve(t reflect.Type, qualifier string) (reflect.Value, error) {
	exact := false // a bean whose own type is a slice/map wins over collection injection
	for _, d := range c.defs {
		if d.Type == t {
			exact = true
			break
		}
	}
	switch {
	case exact:
	case t == containerType: // ApplicationContext injected into a bean
		return reflect.ValueOf(c), nil
	case t.Implements(reflect.TypeFor[binder]()): // ObjectProvider<T>
		p := reflect.New(t.Elem())
		p.Interface().(binder).bind(c)
		return p, nil
	case t.Kind() == reflect.Slice && qualifier == "": // List<T>
		cands := c.candidates(t.Elem())
		sort.SliceStable(cands, func(i, j int) bool {
			if cands[i].Order != cands[j].Order {
				return cands[i].Order < cands[j].Order
			}
			return cands[i].seq < cands[j].seq
		})
		out := reflect.MakeSlice(t, 0, len(cands))
		for _, d := range cands {
			v, err := c.instance(d)
			if err != nil {
				return reflect.Value{}, err
			}
			out = reflect.Append(out, v)
		}
		return out, nil
	case t.Kind() == reflect.Map && t.Key().Kind() == reflect.String && qualifier == "": // Map<String,T>
		out := reflect.MakeMap(t)
		for _, d := range c.candidates(t.Elem()) {
			v, err := c.instance(d)
			if err != nil {
				return reflect.Value{}, err
			}
			out.SetMapIndex(reflect.ValueOf(d.Name), v)
		}
		return out, nil
	}
	d, err := c.single(t, qualifier)
	if err != nil {
		return reflect.Value{}, err
	}
	v, err := c.instance(d)
	if err != nil {
		return reflect.Value{}, err
	}
	if !v.Type().AssignableTo(t) {
		return reflect.Value{}, fmt.Errorf("BeanNotOfRequiredTypeException: Bean named '%s' is "+
			"expected to be of type '%s' but was actually of type '%s'", d.Name, t, v.Type())
	}
	return v, nil
}

// single picks exactly one candidate: @Qualifier, then @Primary, then
// the only non-@Fallback candidate; otherwise the famous exception.
func (c *Container) single(t reflect.Type, qualifier string) (*Definition, error) {
	cands := c.candidates(t)
	if qualifier != "" {
		for _, d := range cands {
			if d.Name == qualifier {
				return d, nil
			}
		}
		return nil, fmt.Errorf("NoSuchBeanDefinitionException: No qualifying bean of type '%s' "+
			"available: expected at least 1 bean which qualifies as autowire candidate. "+
			"Dependency annotations: @Qualifier(\"%s\")", t, qualifier)
	}
	switch len(cands) {
	case 0:
		return nil, fmt.Errorf("NoSuchBeanDefinitionException: No qualifying bean of type '%s' "+
			"available: expected at least 1 bean which qualifies as autowire candidate", t)
	case 1:
		return cands[0], nil
	}
	var primary, regular []*Definition
	for _, d := range cands {
		if d.Primary {
			primary = append(primary, d)
		}
		if !d.Fallback {
			regular = append(regular, d)
		}
	}
	if len(primary) == 1 {
		return primary[0], nil
	}
	if len(primary) > 1 {
		return nil, fmt.Errorf("NoUniqueBeanDefinitionException: more than one 'primary' bean "+
			"found among candidates: %s", names(primary))
	}
	if len(regular) == 1 {
		return regular[0], nil
	}
	return nil, fmt.Errorf("NoUniqueBeanDefinitionException: No qualifying bean of type '%s' "+
		"available: expected single matching bean but found %d: %s", t, len(cands), names(cands))
}

func (c *Container) candidates(t reflect.Type) []*Definition {
	var out []*Definition
	for _, d := range c.defs {
		if d.Type == t || d.Type.AssignableTo(t) {
			out = append(out, d)
		}
	}
	return out
}

func names(ds []*Definition) string {
	n := make([]string, len(ds))
	for i, d := range ds {
		n[i] = d.Name
	}
	return strings.Join(n, ",")
}

// RootCause strips the "Error creating bean" chain, like Spring's
// "Caused by" at the bottom of a stack trace.
func RootCause(err error) error {
	for {
		u := errors.Unwrap(err)
		if u == nil {
			return err
		}
		err = u
	}
}
