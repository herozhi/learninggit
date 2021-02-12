package ratelimit

import "sync/atomic"

// ConcurrencyLimter .
type ConcurrencyLimter struct {
	lim int64
	now int64
	tmp int64
}

// NewConcurrencyLimter .
func NewConcurrencyLimter(lim int64) *ConcurrencyLimter {
	return &ConcurrencyLimter{lim, 0, 0}
}

// TakeOne .
func (ml *ConcurrencyLimter) TakeOne() bool {
	x := atomic.AddInt64(&ml.tmp, 1)
	if x <= atomic.LoadInt64(&ml.lim) {
		atomic.AddInt64(&ml.now, 1)
		return true
	}
	atomic.AddInt64(&ml.tmp, -1)
	return false
}

// ReleaseOne .
func (ml *ConcurrencyLimter) ReleaseOne() {
	atomic.AddInt64(&ml.now, -1)
	atomic.AddInt64(&ml.tmp, -1)
}

// UpdateLimit .
func (ml *ConcurrencyLimter) UpdateLimit(lim int64) {
	atomic.StoreInt64(&ml.lim, lim)
}

// Limit .
func (ml *ConcurrencyLimter) Limit() int64 {
	return atomic.LoadInt64(&ml.lim)
}

// Now .
func (ml *ConcurrencyLimter) Now() int64 {
	return atomic.LoadInt64(&ml.now)
}
