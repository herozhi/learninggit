package connpool

import (
	"fmt"
	"net"
	"time"
)

var _ ConnPool = &ShortPool{}

type shortConn struct {
	net.Conn
	closed bool
}

func (sc *shortConn) Close() error {
	if !sc.closed {
		sc.closed = true
		return sc.Conn.Close()
	}
	return nil
}

// ShortPool .
type ShortPool struct {
	metrics ConnMetrics
	dialer  Dialer
}

// NewShortPool timeout is connection timeout.
func NewShortPool(psm string) *ShortPool {
	return newShortPool(ShortConnMetrics(psm), &netDialer{})
}

func newShortPool(metrics ConnMetrics, dialer Dialer) *ShortPool {
	return &ShortPool{
		metrics: metrics,
		dialer:  dialer,
	}
}

// Get return a PoolConn instance which implemnt net.Conn interface.
func (p *ShortPool) Get(network, address string, connTimeout time.Duration) (net.Conn, error) {
	conn, err := p.dialer.DialTimeout(network, address, connTimeout)
	addr := netAddr{network, address}
	if err != nil {
		p.metrics.ConnFail(addr)
		return nil, fmt.Errorf("dial connection err: %s, addr: %s", err, addr)
	}
	p.metrics.ConnSucc(addr)
	return &shortConn{Conn: conn}, nil
}

func (p *ShortPool) release(conn net.Conn) error {
	if c, ok := conn.(*shortConn); ok {
		return c.Close()
	}
	return nil
}

func (p *ShortPool) Put(conn net.Conn) error {
	return p.release(conn)
}

func (p *ShortPool) Discard(conn net.Conn) error {
	return p.release(conn)
}
