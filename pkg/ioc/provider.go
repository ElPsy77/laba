package ioc

import "reflect"

// Provider is ObjectProvider<T>: inject *ioc.Provider[T] instead of T to
// get a fresh prototype instance on every GetObject() call.
type Provider[T any] struct{ c *Container }

func (p *Provider[T]) bind(c *Container) { p.c = c }

// GetObject returns the bean; for a prototype bean, a new instance each time.
func (p *Provider[T]) GetObject() T {
	v, err := p.c.resolve(reflect.TypeFor[T](), "")
	if err != nil {
		panic(err)
	}
	return v.Interface().(T)
}

// GetIfAvailable returns the bean or the zero value if there is none.
func (p *Provider[T]) GetIfAvailable() (T, bool) {
	v, err := p.c.resolve(reflect.TypeFor[T](), "")
	if err != nil {
		var zero T
		return zero, false
	}
	return v.Interface().(T), true
}
