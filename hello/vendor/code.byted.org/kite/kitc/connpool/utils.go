package connpool

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
)

var (
	_ net.Addr = &netAddr{}

	errRingFull = errors.New("Ring is full")
)

// netAddr implements the net.Addr interface.
type netAddr struct {
	network string
	address string
}

func (na netAddr) Network() string { return na.network }
func (na netAddr) String() string  { return na.address }

// ConnwithPkgSize wraps a connection and records the bytes read or written through it.
type ConnWithPkgSize struct {
	net.Conn
	Written int32
	Readn   int32
}

func (c *ConnWithPkgSize) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)
	atomic.AddInt32(&(c.Readn), int32(n))
	return n, err
}

func (c *ConnWithPkgSize) Write(b []byte) (n int, err error) {
	n, err = c.Conn.Write(b)
	atomic.AddInt32(&(c.Written), int32(n))
	return n, err
}

func (c *ConnWithPkgSize) Close() error {
	err := c.Conn.Close()
	c.Conn = nil
	return err
}

// CountLimiter.
type CountLimiter struct {
	now int64
	Max int
}

func (cl *CountLimiter) Inc() bool {
	if atomic.AddInt64(&cl.now, 1) > int64(cl.Max) {
		atomic.AddInt64(&cl.now, -1)
		return false
	}
	return true
}

func (cl *CountLimiter) Dec() {
	atomic.AddInt64(&cl.now, -1)
}

// Ring implements a fixed size ring buffer to manage data
type Ring struct {
	l    sync.Mutex
	arr  []interface{}
	size int
	tail int
	head int
}

func NewRing(size int) *Ring {
	return &Ring{
		arr:  make([]interface{}, size+1),
		size: size,
	}
}

func (r *Ring) Push(i interface{}) error {
	r.l.Lock()
	defer r.l.Unlock()
	if r.isFull() {
		return errRingFull
	}
	r.arr[r.head] = i
	r.head = r.inc()
	return nil
}

func (r *Ring) Pop() interface{} {
	r.l.Lock()
	defer r.l.Unlock()
	if r.isEmpty() {
		return nil
	}
	c := r.arr[r.tail]
	r.arr[r.tail] = nil
	r.tail = r.dec()
	return c
}

func (r *Ring) inc() int {
	return (r.head + 1) % (r.size + 1)
}

func (r *Ring) dec() int {
	return (r.tail + 1) % (r.size + 1)
}

func (r *Ring) isEmpty() bool {
	return r.tail == r.head
}

func (r *Ring) isFull() bool {
	return r.inc() == r.tail
}
