package trace

import (
	"sync"
	"time"

	"code.byted.org/trace/trace-client-go/jaeger-client/utils"
)

var (
	// 总span 默认限制 100个/s，
	defaultSampledSpansRateLimitPerSecond             float64 = 6000.0 / 60
	defaultSampledSpansRateLimitMaxBalance            float64 = 6000.0
	defaultSampledSpansRateLimitUpdateBalanceInterval int64   = 60
	defaultSampledSpansRateLimit                              = newRateLimitParam(
		defaultSampledSpansRateLimitPerSecond,
		defaultSampledSpansRateLimitMaxBalance,
		defaultSampledSpansRateLimitUpdateBalanceInterval)

	// post span 默认限制 300个/5分钟，
	defaultPostSpansRateLimitPerSecond             float64 = 300.0 / 300
	defaultPostSpansRateLimitMaxBalance            float64 = 300.0
	defaultPostSpansRateLimitUpdateBalanceInterval int64   = 300
	defaultPostSpansRateLimit                              = newRateLimitParam(
		defaultPostSpansRateLimitPerSecond,
		defaultPostSpansRateLimitMaxBalance,
		defaultPostSpansRateLimitUpdateBalanceInterval)

	// post span 单个下游默认限制 10个/5分钟，
	defaultDownstreamPostSpansRateLimitMaxPeers              int     = 2000
	defaultDownstreamPostSpansRateLimitPerSecond             float64 = 10.0 / 300
	defaultDownstreamPostSpansRateLimitMaxBalance            float64 = 10.0
	defaultDownstreamPostSpansRateLimitUpdateBalanceInterval int64   = 300
	defaultDownstreamPostSpansRateLimit                              = newRateLimitParam(
		defaultDownstreamPostSpansRateLimitPerSecond,
		defaultDownstreamPostSpansRateLimitMaxBalance,
		defaultDownstreamPostSpansRateLimitUpdateBalanceInterval)
)

var globalLimiter = newSpanLimiter(0,
	defaultSampledSpansRateLimitPerSecond,
	defaultSampledSpansRateLimitMaxBalance,
	defaultSampledSpansRateLimitUpdateBalanceInterval)

var globalPostSpansLimiter = newSpanLimiter(0,
	defaultPostSpansRateLimitPerSecond,
	defaultPostSpansRateLimitMaxBalance,
	defaultPostSpansRateLimitUpdateBalanceInterval)

var downstreamPostSpansLimiter = newSpanLimiter(defaultDownstreamPostSpansRateLimitMaxPeers,
	defaultDownstreamPostSpansRateLimitPerSecond,
	defaultDownstreamPostSpansRateLimitMaxBalance,
	defaultDownstreamPostSpansRateLimitUpdateBalanceInterval)

func newRateLimitParam(rateLimitPerSecond, maxBalance float64, updateBalanceInterval int64) *RateLimitParam {
	return &RateLimitParam{
		RateLimitPerSecond:    &rateLimitPerSecond,
		MaxBalance:            &maxBalance,
		UpdateBalanceInterval: &updateBalanceInterval,
	}
}

type RateLimitParam struct {
	RateLimitPerSecond    *float64 `json:"rate_limit"`
	MaxBalance            *float64 `json:"max_balance"`
	UpdateBalanceInterval *int64   `json:"update_balance_interval"`
}

func (p *RateLimitParam) IsValid() bool {
	return p != nil && p.RateLimitPerSecond != nil
}

func (p *RateLimitParam) GetRateLimitPerSecond() float64 {
	if p != nil {
		if p.RateLimitPerSecond != nil {
			return *p.RateLimitPerSecond
		}
	}
	return -1
}

func (p *RateLimitParam) GetMaxBalance() float64 {
	if p != nil {
		if p.MaxBalance != nil {
			return *p.MaxBalance
		}
	}
	return p.GetRateLimitPerSecond()
}

func (p *RateLimitParam) GetUpdateBalanceInterval() time.Duration {
	if p != nil {
		if p.UpdateBalanceInterval != nil {
			return time.Duration(*p.UpdateBalanceInterval) * time.Second
		}
	}
	return time.Duration(0)
}

func (p *RateLimitParam) Equals(q *RateLimitParam) bool {
	return p.GetRateLimitPerSecond() == q.GetRateLimitPerSecond() &&
		p.GetMaxBalance() == q.GetMaxBalance() &&
		p.GetUpdateBalanceInterval() == q.GetUpdateBalanceInterval()
}

type upperBoundLimiter struct {
	rateLimitParam *RateLimitParam
	rateLimiter    utils.RateLimiter
}

func (limiter *upperBoundLimiter) IsLimited(itemCost float64) bool {
	if limiter != nil && limiter.rateLimitParam.GetRateLimitPerSecond() >= 0.0 && limiter.rateLimiter != nil {
		return !limiter.rateLimiter.CheckCredit(itemCost)
	}
	return false
}

func newSpanLimiter(maxOperations int, rateLimitPerSecond, maxBalance float64, updateBalanceInterval int64) *spanLimiter {
	l := new(spanLimiter)
	l.update(maxOperations, newRateLimitParam(rateLimitPerSecond, maxBalance, updateBalanceInterval), nil)
	return l
}

// spanLimiter used to control span reporting qps
type spanLimiter struct {
	sync.RWMutex

	maxOperations int

	defaultRateLimitParam *RateLimitParam
	opsRateLimitParam     map[string]*RateLimitParam

	defaultLimiter *upperBoundLimiter
	opsLimiter     map[string]*upperBoundLimiter
}

func (l *spanLimiter) IsBlock(operation string) bool {
	l.RLock()
	limiter, ok := l.opsLimiter[operation]
	if ok {
		defer l.RUnlock()
		return limiter.IsLimited(1.0)
	}

	// Store only up to maxOperations of unique ops.
	if len(l.opsLimiter) >= l.maxOperations {
		defer l.RUnlock()
		return l.defaultLimiter.IsLimited(1.0)
	}
	l.RUnlock()

	l.Lock()
	defer l.Unlock()

	// Check if limiter has already been created
	limiter, ok = l.opsLimiter[operation]
	if ok {
		return limiter.IsLimited(1.0)
	}

	// Store only up to maxOperations of unique ops.
	if len(l.opsLimiter) >= l.maxOperations {
		return l.defaultLimiter.IsLimited(1.0)
	}
	newLimiter := newUpperBoundLimiter(l.defaultRateLimitParam)
	l.opsLimiter[operation] = newLimiter
	return newLimiter.IsLimited(1.0)
}

func (l *spanLimiter) update(maxOperations int, defaultUpperBound *RateLimitParam, opsUpperBound map[string]*RateLimitParam) {
	l.Lock()
	defer l.Unlock()
	newOpsLimiter := make(map[string]*upperBoundLimiter)

	for op, ub := range opsUpperBound {
		oldParam, ok := l.opsRateLimitParam[op]
		if ok && oldParam.Equals(ub) && l.opsLimiter[op] != nil {
			newOpsLimiter[op] = l.opsLimiter[op]
		} else {
			newOpsLimiter[op] = newUpperBoundLimiter(ub)
		}
	}

	// 若default限流没变的话，把原有的default参数产生的limiter，copy到新的map中
	if l.defaultLimiter != nil && l.defaultRateLimitParam.Equals(defaultUpperBound) {
		for op, limiter := range l.opsLimiter {
			if len(newOpsLimiter) >= maxOperations {
				break
			}

			// 新配置已显式配置
			if newOpsLimiter[op] != nil {
				continue
			}

			// 无效limiter
			if limiter == nil {
				continue
			}

			// 老配置中显式配置，新配置中不存在的
			if _, ok := l.opsRateLimitParam[op]; ok {
				continue
			}

			// 这里相等比较，直接用指针，正常就应该是同个对象
			if limiter.rateLimitParam == l.defaultRateLimitParam {
				newOpsLimiter[op] = limiter
			}
		}
	} else {
		l.defaultLimiter = newUpperBoundLimiter(defaultUpperBound)
		l.defaultRateLimitParam = defaultUpperBound
	}

	l.opsLimiter = newOpsLimiter
	l.opsRateLimitParam = opsUpperBound
	l.maxOperations = maxOperations
}

func newUpperBoundLimiter(param *RateLimitParam) *upperBoundLimiter {
	return &upperBoundLimiter{
		rateLimitParam: param,
		rateLimiter:    utils.NewRateLimiter(param.GetRateLimitPerSecond(), param.GetMaxBalance(), param.GetUpdateBalanceInterval()),
	}
}
