package aspect

import (
	"fmt"
	"log/slog"
	"time"

	"springlab/internal/audit"
	"springlab/pkg/aop"
	"springlab/pkg/ioc"
)

// AuditAspect — @Aspect @Component @Order(1): the outermost layer, first
// on the way in and last on the way out.
func init() {
	ioc.Component(NewAuditAspect, ioc.Order(1))
}

type AuditAspect struct{}

func NewAuditAspect() *AuditAspect { return &AuditAspect{} }

func (a *AuditAspect) Advices() []aop.Advice {
	// @Around("@annotation(audited)"): the name in the expression is bound
	// to the advice parameter of type audit.Audited — the annotation
	// instance itself, with all its attributes, arrives in the advice.
	return []aop.Advice{aop.AroundAnnotated("@annotation(audited)", a.audit)}
}

func (a *AuditAspect) audit(pjp *aop.ProceedingJoinPoint, audited audit.Audited) (any, error) {
	start := fmt.Sprintf("[AUDIT] start %s", audited.Action)
	if audited.LogArguments {
		start += fmt.Sprintf(" args=%v", pjp.Args())
	}
	slog.Info(start)

	result, err := pjp.Proceed()
	at := time.Now().Format("2006-01-02T15:04:05.000")
	if err != nil {
		slog.Warn(fmt.Sprintf("[AUDIT] %s failure at %s: %s", audited.Action, at, err))
		return nil, err // the error is not suppressed: it is "rethrown"
	}
	slog.Info(fmt.Sprintf("[AUDIT] %s success at %s", audited.Action, at))
	return result, nil
}
