package aspect

import (
	"fmt"
	"log/slog"

	"springlab/pkg/aop"
	"springlab/pkg/ioc"
)

// LoggingAspect — Listing 3: @Aspect @Component @Order(2). Registering it as
// a component is what makes it work: without this line the advice
// silently never runs — "the code does not work, there are no errors".
func init() {
	ioc.Component(NewLoggingAspect, ioc.Order(2))
}

type LoggingAspect struct{}

func NewLoggingAspect() *LoggingAspect { return &LoggingAspect{} }

func (a *LoggingAspect) Advices() []aop.Advice {
	return []aop.Advice{
		aop.Before(ServiceOperation, a.before),
		aop.AfterReturning(ServiceOperation, a.afterReturning), // returning = "result"
		aop.AfterThrowing(ServiceOperation, a.afterThrowing),   // throwing = "ex"
	}
}

// @Before: the method name and the arguments.
func (a *LoggingAspect) before(jp *aop.JoinPoint) {
	slog.Info(fmt.Sprintf("[LOG] -> %s args=%v", jp.Signature().ToShortString(), jp.Args()))
}

// @AfterReturning: the method name and the returned value.
func (a *LoggingAspect) afterReturning(jp *aop.JoinPoint, result any) {
	slog.Info(fmt.Sprintf("[LOG] <- %s returned %v", jp.Signature().Name(), result))
}

// @AfterThrowing: the type and the message of the error. The error is not
// suppressed — it travels on to the caller.
func (a *LoggingAspect) afterThrowing(jp *aop.JoinPoint, ex error) {
	slog.Error(fmt.Sprintf("[LOG] !! %s threw %T: %s", jp.Signature().Name(), ex, ex.Error()))
}
