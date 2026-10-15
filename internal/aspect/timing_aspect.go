package aspect

import (
	"fmt"
	"log/slog"
	"time"

	"springlab/pkg/aop"
	"springlab/pkg/ioc"
)

// TimingAspect — Listing 4: @Aspect @Component @Order(3).
func init() {
	ioc.Component(NewTimingAspect, ioc.Order(3))
}

type TimingAspect struct{}

func NewTimingAspect() *TimingAspect { return &TimingAspect{} }

func (a *TimingAspect) Advices() []aop.Advice {
	return []aop.Advice{aop.Around(ServiceOperation, a.measure)}
}

// measure — @Around: measures the execution time and warns above 200 ms.
func (a *TimingAspect) measure(pjp *aop.ProceedingJoinPoint) (any, error) {
	started := time.Now()
	defer func() { // finally
		ms := time.Since(started).Milliseconds()
		name := pjp.Signature().ToShortString()
		if ms > 200 {
			slog.Warn(fmt.Sprintf("[TIME] SLOW: %s — %d ms", name, ms))
		} else {
			slog.Info(fmt.Sprintf("[TIME] %s — %d ms", name, ms))
		}
	}()
	return pjp.Proceed() // without this line the method never runs
}
