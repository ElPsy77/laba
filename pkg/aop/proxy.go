package aop

import (
	"fmt"
	"log/slog"
	"reflect"
	"sync"

	"springlab/pkg/ioc"
)

// Proxy is implemented by every generated proxy (Spring: Advised).
type Proxy interface {
	TargetSource() any
}

// IsAopProxy — AopUtils.isAopProxy(bean).
func IsAopProxy(bean any) bool {
	_, ok := bean.(Proxy)
	return ok
}

// TargetOf returns the object behind a proxy (or the bean itself).
func TargetOf(bean any) any {
	if p, ok := bean.(Proxy); ok {
		return p.TargetSource()
	}
	return bean
}

// SelfAware is implemented by a bean that wants a reference to its own
// proxy — the Go counterpart of "@Autowired private CatalogService self".
type SelfAware interface {
	SetSelf(proxy any)
}

// ProxyFactory is emitted by the generator for each proxied type.
type ProxyFactory struct {
	Methods []*Method
	New     func(target any, h *Handler) any
}

var (
	factoriesMu sync.Mutex
	factories   = map[reflect.Type]ProxyFactory{}
)

// RegisterProxy is called from the generated *_aop.go file.
func RegisterProxy(target reflect.Type, f ProxyFactory) {
	factoriesMu.Lock()
	defer factoriesMu.Unlock()
	factories[target] = f
}

// ---- the interceptor chain -------------------------------------------------------

type aspectAdvices struct {
	name    string
	advices []*Advice // matching this method, in declaration order
}

// Handler is held by a proxy and runs the chain for each call.
type Handler struct {
	bean   string
	target any
	proxy  any
	chains map[string][]aspectAdvices
}

// SetProxy is called by the generated factory once the proxy exists.
func (h *Handler) SetProxy(p any) { h.proxy = p }

// Invoke runs the advice chain around call. Aspects are nested by @Order:
// the lowest value is the outermost layer — first in, last out.
func (h *Handler) Invoke(m *Method, args []any, call func(args []any) (any, error)) (any, error) {
	chain := h.chains[m.Name]
	if len(chain) == 0 {
		return call(args)
	}
	jp := &JoinPoint{Method: m, target: h.target, this: h.proxy, args: args}
	var run func(i int, args []any) (any, error)
	run = func(i int, args []any) (any, error) {
		if i == len(chain) {
			jp.args = args
			return call(args)
		}
		return invokeAspect(chain[i].advices, jp, args, func(a []any) (any, error) { return run(i+1, a) })
	}
	return run(0, args)
}

// invokeAspect applies the advice of ONE aspect. Inside an aspect the
// order is fixed: @Around (before proceed) -> @Before -> method ->
// @AfterReturning | @AfterThrowing -> @After -> @Around (after proceed).
func invokeAspect(advices []*Advice, jp *JoinPoint, args []any,
	next func([]any) (any, error)) (any, error) {

	core := func(args []any) (any, error) {
		jp.args = args
		for _, a := range advices {
			if a.Kind == KindBefore {
				a.before(jp)
			}
		}
		res, err := next(args)
		for _, a := range advices {
			switch {
			case a.Kind == KindAfterThrowing && err != nil:
				a.afterThrowing(jp, err)
			case a.Kind == KindAfterReturning && err == nil:
				a.afterReturning(jp, res)
			}
		}
		for _, a := range advices {
			if a.Kind == KindAfter {
				a.after(jp)
			}
		}
		return res, err
	}
	// several @Around advice in one aspect: the first declared is outermost
	call := core
	for i := len(advices) - 1; i >= 0; i-- {
		if a := advices[i]; a.Kind == KindAround {
			inner := call
			call = func(args []any) (any, error) {
				jp.args = args
				return a.around(&ProceedingJoinPoint{JoinPoint: jp, next: inner})
			}
		}
	}
	return call(args)
}

// ---- auto-proxy creator -------------------------------------------------------------

// AutoProxyCreator is the BeanPostProcessor that turns @Aspect beans into
// behaviour: after a bean has been initialised (@PostConstruct), it wraps
// the bean in a proxy if any pointcut matches any of its methods.
type AutoProxyCreator struct {
	ctx     *ioc.Container
	once    sync.Once
	err     error
	aspects []namedAspect
}

type namedAspect struct {
	name   string
	aspect Aspect
}

func NewAutoProxyCreator(ctx *ioc.Container) *AutoProxyCreator {
	return &AutoProxyCreator{ctx: ctx}
}

func (a *AutoProxyCreator) loadAspects() error {
	a.once.Do(func() {
		beans, err := ioc.BeansOfType[Aspect](a.ctx) // ordered by @Order
		if err != nil {
			a.err = err
			return
		}
		for _, b := range beans {
			name := reflect.TypeOf(b).Elem().Name()
			advs := b.Advices()
			for i := range advs {
				if err := advs[i].compile(); err != nil {
					a.err = fmt.Errorf("IllegalArgumentException: %s %s(%q): %w",
						name, advs[i].Kind, advs[i].Expr, err)
					return
				}
			}
			a.aspects = append(a.aspects, namedAspect{name, wrapAspect{b, advs}})
		}
	})
	return a.err
}

type wrapAspect struct {
	Aspect
	advs []Advice
}

func (w wrapAspect) Advices() []Advice { return w.advs }

// PostProcessAfterInitialization implements ioc.BeanPostProcessor.
func (a *AutoProxyCreator) PostProcessAfterInitialization(bean any, name string) (any, error) {
	if _, isAspect := bean.(Aspect); isAspect {
		return bean, nil // aspects are not advised themselves
	}
	factoriesMu.Lock()
	f, ok := factories[reflect.TypeOf(bean)]
	factoriesMu.Unlock()
	if !ok {
		return bean, nil // nothing generated for this type: it cannot be proxied
	}
	if err := a.loadAspects(); err != nil {
		return nil, err
	}
	h := &Handler{bean: name, target: bean, chains: map[string][]aspectAdvices{}}
	for _, m := range f.Methods {
		for _, as := range a.aspects {
			var matched []*Advice
			advs := as.aspect.Advices()
			for i := range advs {
				if advs[i].pointcut.Matches(m, name) {
					matched = append(matched, &advs[i])
				}
			}
			if len(matched) > 0 {
				h.chains[m.Name] = append(h.chains[m.Name], aspectAdvices{as.name, matched})
			}
		}
	}
	if len(h.chains) == 0 {
		return bean, nil // no advice applies: no proxy is created
	}
	proxy := f.New(bean, h)
	slog.Debug(fmt.Sprintf("Creating AOP proxy for bean '%s': %T -> %T", name, bean, proxy))
	if s, ok := bean.(SelfAware); ok {
		s.SetSelf(proxy) // the proxy is placed into the "self" field
	}
	return proxy, nil
}

// Adding the "starter" (importing this package) switches AOP on, like
// spring-boot-starter-aop does: @EnableAspectJAutoProxy is not needed.
func init() {
	ioc.AutoConfiguration("AopAutoConfiguration", NewAutoProxyCreator,
		ioc.OnProperty("spring.aop.auto", "true", true))
}
