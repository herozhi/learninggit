package goredis

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// SafeRander is used for avoiding to use global's rand;
type SafeRander struct {
	pos     uint32
	randers [128]*rand.Rand
	locks   [128]*sync.Mutex
}

// NewSafeRander .
func NewSafeRander() *SafeRander {
	var randers [128]*rand.Rand
	var locks [128]*sync.Mutex
	for i := 0; i < 128; i++ {
		randers[i] = rand.New(rand.NewSource(time.Now().UnixNano()))
		locks[i] = new(sync.Mutex)
	}
	return &SafeRander{
		randers: randers,
		locks:   locks,
	}
}

// Intn .
func (sr *SafeRander) Intn(n int) int {
	x := atomic.AddUint32(&sr.pos, 1)
	x %= 128
	sr.locks[x].Lock()
	n = sr.randers[x].Intn(n)
	sr.locks[x].Unlock()
	return n
}

var defaultSafeRander = NewSafeRander()
