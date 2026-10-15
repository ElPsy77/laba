package aspect

import (
	"sort"
	"sync"
	"sync/atomic"

	"springlab/pkg/aop"
	"springlab/pkg/ioc"
)

// CallCounterAspect — individual assignment, variant 1: @Before advice on
// the service-layer pointcut counts calls per method name. sync.Map plus
// atomic counters is the Go counterpart of ConcurrentHashMap: the aspect is
// a singleton called from many goroutines at once.
func init() {
	ioc.Component(NewCallCounterAspect, ioc.Order(4))
}

type CallCounterAspect struct {
	counts sync.Map // method name -> *atomic.Int64
}

func NewCallCounterAspect() *CallCounterAspect { return &CallCounterAspect{} }

func (a *CallCounterAspect) Advices() []aop.Advice {
	return []aop.Advice{
		aop.Before("springlab.internal.aspect.Pointcuts.serviceLayer()", a.count),
	}
}

func (a *CallCounterAspect) count(jp *aop.JoinPoint) {
	c, _ := a.counts.LoadOrStore(jp.Signature().ToShortString(), new(atomic.Int64))
	c.(*atomic.Int64).Add(1)
}

// MethodStat is one row of the statistics.
type MethodStat struct {
	Method string `json:"method"`
	Calls  int64  `json:"calls"`
}

// Stats returns the counters sorted by method name.
func (a *CallCounterAspect) Stats() []MethodStat {
	out := []MethodStat{}
	a.counts.Range(func(k, v any) bool {
		out = append(out, MethodStat{k.(string), v.(*atomic.Int64).Load()})
		return true
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Method < out[j].Method })
	return out
}
