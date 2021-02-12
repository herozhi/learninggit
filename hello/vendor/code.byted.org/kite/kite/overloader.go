package kite

import (
	"sync"
	"time"

	"code.byted.org/kite/kite/ratelimit"
)

type endpointQPSLimter struct {
	limiter *ratelimit.QPSLimiter
}

// overloader protect kite from overload
type overloader struct {
	connLimiter            *ratelimit.ConcurrencyLimter
	qpsLimiter             *ratelimit.QPSLimiter
	endpointQPSLimiter     map[string]endpointQPSLimter
	endpointQPSLimiterLock sync.RWMutex // for update endpointLimiter
}

func newOverloader(connLim, qpsLim int64, qpsInterval time.Duration) *overloader {
	return &overloader{
		connLimiter:        ratelimit.NewConcurrencyLimter(connLim),
		qpsLimiter:         ratelimit.NewQPSLimiter(qpsInterval, int32(qpsLim)),
		endpointQPSLimiter: make(map[string]endpointQPSLimter),
	}
}

func (ol *overloader) TakeConn() bool {
	return ol.connLimiter.TakeOne()
}

func (ol *overloader) ReleaseConn() {
	ol.connLimiter.ReleaseOne()
}

func (ol *overloader) UpdateConnLimit(lim int64) {
	ol.connLimiter.UpdateLimit(lim)
}

func (ol *overloader) ConnNow() int64 {
	return ol.connLimiter.Now()
}

func (ol *overloader) ConnLimit() int64 {
	return ol.connLimiter.Limit()
}

func (ol *overloader) TakeQPS() bool {
	return ol.qpsLimiter.Take()
}

func (ol *overloader) UpdateQPSLimit(lim int64) {
	ol.qpsLimiter.UpdateLimiter(ol.qpsLimiter.Interval(), int32(lim))
}

func (ol *overloader) QPSLimit() int64 {
	return int64(ol.qpsLimiter.QPSLimit())
}

func (ol *overloader) TakeEndpointQPS(endpoint string) bool {
	var ok bool
	ol.endpointQPSLimiterLock.RLock()
	if l, exist := ol.endpointQPSLimiter[endpoint]; exist {
		ok = l.limiter.Take()
	} else {
		ok = true
	}
	ol.endpointQPSLimiterLock.RUnlock()
	return ok
}

func (ol *overloader) UpdateEndpointQPSLimit(endPointQPSLimit map[string]int) {
	// fast path for most case, no race condition due to no change on the map
	if len(ol.endpointQPSLimiter) == 0 && len(endPointQPSLimit) == 0 {
		return
	}

	ol.endpointQPSLimiterLock.Lock()
	// remove deleted endpoint qps limit
	for endpoint := range ol.endpointQPSLimiter {
		if _, exist := endPointQPSLimit[endpoint]; !exist {
			delete(ol.endpointQPSLimiter, endpoint)
		}
	}
	// add or update new endpoint qps limit
	// NOTE: if the qps limit is 0, we treat it as no limit, if you want to disable this method,
	// use ACL. the web frontend should give tips on this
	for endpoint, qpsLimit := range endPointQPSLimit {
		l, exist := ol.endpointQPSLimiter[endpoint]
		if !exist || int(l.limiter.QPSLimit()) != qpsLimit {
			if qpsLimit > 0 {
				l.limiter = ratelimit.NewQPSLimiter(limitQPSInterval, int32(qpsLimit))
				ol.endpointQPSLimiter[endpoint] = l
			} else if exist {
				delete(ol.endpointQPSLimiter, endpoint)
			}
		}
	}
	ol.endpointQPSLimiterLock.Unlock()
}
