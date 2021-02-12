package kitevent

import (
	"sync"
	"sync/atomic"
)

type Queue interface {
	Push(e *KitEvent)
	Dump() []*KitEvent
}

// 这个queue的实现方法是通过CAS来进行下标的增加
// 由于并发情况之下同一时间一定有一个goroutine会CAS成功
// 所以不会一直在for循环中
// 潜在的问题是有可能在极端情况之下，在一瞬间增加了cap个event
// 这个时候，可能会导致新来的event覆盖后来的event
// 但是发生的概率极低，几乎不可能，所以没有采用两次CAS来保证一定不会发生这种情况
// 在CAS的时候进行RLock，因为允许多个routine一起CAS，比旧版本直接加Lock性能强很多
// Dump的时候加Lock就可以保证Dump的时候不会被修改

// Queue .
type queue struct {
	ring []*KitEvent
	tail int32
	mu   sync.RWMutex
}

// NewQueue .
func NewQueue(cap int) Queue {
	return &queue{
		ring: make([]*KitEvent, cap),
	}
}

// Push .
func (q *queue) Push(e *KitEvent) {
	for {
		old := atomic.LoadInt32(&q.tail)
		new := old + 1
		if new >= int32(len(q.ring)) {
			new = 0
		}
		if atomic.CompareAndSwapInt32(&q.tail, old, new) {
			q.mu.RLock()
			q.ring[old] = e
			q.mu.RUnlock()
			break
		}
	}
}

// Dump .
func (q *queue) Dump() []*KitEvent {
	results := make([]*KitEvent, 0, len(q.ring))
	q.mu.Lock()
	defer q.mu.Unlock()
	pos := atomic.LoadInt32(&q.tail)
	for i := 0; i < len(q.ring); i++ {
		pos--
		if pos < 0 {
			pos = int32(len(q.ring) - 1)
		}

		e := q.ring[pos]
		if e == nil {
			return results
		}

		results = append(results, e)
	}

	return results
}
