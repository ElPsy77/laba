package aop

import "reflect"

// Kind is one of the five advice types.
type Kind int

const (
	KindAround Kind = iota
	KindBefore
	KindAfterReturning
	KindAfterThrowing
	KindAfter
)

func (k Kind) String() string {
	return [...]string{"@Around", "@Before", "@AfterReturning", "@AfterThrowing", "@After"}[k]
}

// Aspect is implemented by every aspect bean (@Aspect). Like in Spring, an
// aspect is found only if it is also registered as a component — an
// @Aspect without @Component silently does nothing.
type Aspect interface {
	Advices() []Advice
}

// Advice is one advice method of an aspect: a pointcut ("where") plus an
// action ("what to do").
type Advice struct {
	Kind     Kind
	Expr     string
	binding  reflect.Type
	pointcut *Pointcut

	before         func(*JoinPoint)
	afterReturning func(*JoinPoint, any)
	afterThrowing  func(*JoinPoint, error)
	after          func(*JoinPoint)
	around         func(*ProceedingJoinPoint) (any, error)
}

// Before — @Before: runs before the method; cannot cancel the call
// (it may only panic).
func Before(expr string, fn func(jp *JoinPoint)) Advice {
	return Advice{Kind: KindBefore, Expr: expr, before: fn}
}

// AfterReturning — @AfterReturning(returning = "result"): receives the
// result but cannot replace it.
func AfterReturning(expr string, fn func(jp *JoinPoint, result any)) Advice {
	return Advice{Kind: KindAfterReturning, Expr: expr, afterReturning: fn}
}

// AfterThrowing — @AfterThrowing(throwing = "ex"): receives the error;
// does not suppress it, it travels on to the caller.
func AfterThrowing(expr string, fn func(jp *JoinPoint, err error)) Advice {
	return Advice{Kind: KindAfterThrowing, Expr: expr, afterThrowing: fn}
}

// After — @After: runs in either case, like finally.
func After(expr string, fn func(jp *JoinPoint)) Advice {
	return Advice{Kind: KindAfter, Expr: expr, after: fn}
}

// Around — @Around: replaces the call and decides whether to Proceed().
func Around(expr string, fn func(pjp *ProceedingJoinPoint) (any, error)) Advice {
	return Advice{Kind: KindAround, Expr: expr, around: fn}
}

// AroundAnnotated — @Around("@annotation(audited)") with the annotation
// instance as the second parameter: the binding is by the parameter type A.
func AroundAnnotated[A any](expr string, fn func(pjp *ProceedingJoinPoint, ann A) (any, error)) Advice {
	t := reflect.TypeFor[A]()
	return Advice{Kind: KindAround, Expr: expr, binding: t,
		around: func(pjp *ProceedingJoinPoint) (any, error) {
			ann, _ := pjp.Method.annotationOfType(t)
			return fn(pjp, ann.(A))
		}}
}

// BeforeAnnotated — @Before("@annotation(x)") with the annotation instance.
func BeforeAnnotated[A any](expr string, fn func(jp *JoinPoint, ann A)) Advice {
	t := reflect.TypeFor[A]()
	return Advice{Kind: KindBefore, Expr: expr, binding: t,
		before: func(jp *JoinPoint) {
			ann, _ := jp.Method.annotationOfType(t)
			fn(jp, ann.(A))
		}}
}

func (a *Advice) compile() error {
	if a.pointcut != nil {
		return nil
	}
	pc, err := compile(a.Expr, a.binding, "")
	if err != nil {
		return err
	}
	a.pointcut = pc
	return nil
}
