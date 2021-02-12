package connpool

import (
	"errors"
	"net"
	"sync"
	"time"
)

var (
	errPeerNotInit = errors.New("Peer is not initialized")

	_ net.Conn     = &longConn{}
	_ LongConnPool = &LongPool{}
)

// longConn implements the net.Conn interface.
type longConn struct {
	net.Conn
	sync.RWMutex
	err      error
	deadline time.Time
}

func (c *longConn) hasError() bool {
	return c.err != nil
}

func (c *longConn) markError(err error) {
	if err != nil {
		c.Lock()
		c.err = err
		c.Unlock()
	}
}

func (c *longConn) Close() error {
	return nil
}

func (c *longConn) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)
	c.markError(err)
	return n, err
}

func (c *longConn) Write(b []byte) (n int, err error) {
	n, err = c.Conn.Write(b)
	c.markError(err)
	return n, err
}

func (c *longConn) SetDeadline(t time.Time) error {
	err := c.Conn.SetDeadline(t)
	c.markError(err)
	return err
}

func (c *longConn) SetReadDeadline(t time.Time) error {
	err := c.Conn.SetReadDeadline(t)
	c.markError(err)
	return err
}

func (c *longConn) SetWriteDeadline(t time.Time) error {
	err := c.Conn.SetWriteDeadline(t)
	c.markError(err)
	return err
}

// Peer has one address, it manage all connections base on this address
type peer struct {
	dialer         Dialer
	addr           net.Addr
	ring           *Ring
	globalIdle     *CountLimiter
	maxIdleTimeout time.Duration
	metrics        ConnMetrics
}

func newPeer(
	addr net.Addr,
	maxIdle int,
	maxIdleTimeout time.Duration,
	globalIdle *CountLimiter,
	metrics ConnMetrics,
	dialer Dialer,
) *peer {
	return &peer{
		dialer:         dialer,
		addr:           addr,
		ring:           NewRing(maxIdle),
		globalIdle:     globalIdle,
		maxIdleTimeout: maxIdleTimeout,
		metrics:        metrics,
	}
}

func (p *peer) Reset(addr net.Addr) {
	p.addr = addr
	p.Close()
}

func (p *peer) Get(timeout time.Duration) (net.Conn, error) {
	// pick up connection from ring
	for {
		conn, _ := p.ring.Pop().(*longConn)
		if conn == nil {
			break
		}
		p.globalIdle.Dec()
		if time.Now().Before(conn.deadline) {
			p.metrics.ReuseSucc(p.addr)
			return conn, nil
		}
		// close connection after deadline
		conn.Conn.Close()
	}

	conn, err := p.dialer.DialTimeout(p.addr.Network(), p.addr.String(), timeout)
	if err != nil {
		p.metrics.ConnFail(p.addr)
		return nil, err
	}
	p.metrics.ConnSucc(p.addr)
	return &longConn{Conn: conn, deadline: time.Now().Add(p.maxIdleTimeout)}, nil
}

func (p *peer) put(c *longConn) error {
	if c.hasError() {
		return c.Conn.Close()
	}
	if !p.globalIdle.Inc() {
		return c.Conn.Close()
	}
	c.deadline = time.Now().Add(p.maxIdleTimeout)
	err := p.ring.Push(c)
	if err != nil {
		p.globalIdle.Dec()
		return c.Conn.Close()
	}
	return nil
}

func (p *peer) Close() {
	for {
		conn, _ := p.ring.Pop().(*longConn)
		if conn == nil {
			break
		}
		p.globalIdle.Dec()
		conn.Conn.Close()
	}
}

// LongPool manages a pool of long connections.
type LongPool struct {
	peerMap sync.Map
	newPeer func(net.Addr) *peer
}

func (lp *LongPool) getPeer(addr netAddr) *peer {
	if p, ok := lp.peerMap.Load(addr); ok {
		return p.(*peer)
	} else {
		p, _ := lp.peerMap.LoadOrStore(addr, lp.newPeer(addr))
		return p.(*peer)
	}
}

// Get pick or generate a net.Conn and return
func (lp *LongPool) Get(network, address string, connTimeout time.Duration) (net.Conn, error) {
	addr := netAddr{network, address}
	p := lp.getPeer(addr)
	return p.Get(connTimeout)
}

func (lp *LongPool) Put(conn net.Conn) error {
	if c, ok := conn.(*longConn); ok {
		addr := conn.RemoteAddr()
		na := netAddr{addr.Network(), addr.String()}
		if p, ok := lp.peerMap.Load(na); ok {
			p.(*peer).put(c)
			return nil
		} else {
			return c.Conn.Close()
		}
	} else {
		// TODO: should we return error instead?
		return conn.Close()
	}
}

func (lp *LongPool) Clean(network, address string) {
	na := netAddr{network, address}
	if p, ok := lp.peerMap.Load(na); ok {
		lp.peerMap.Delete(na)
		go p.(*peer).Close()
	}
}

func (lp *LongPool) Discard(conn net.Conn) error {
	if c, ok := conn.(*longConn); ok {
		return c.Conn.Close()
	} else {
		return conn.Close()
	}
}

// NewLongPool .
func NewLongPool(maxIdlePerIns, maxIdleGlobal int, maxIdleTimeout time.Duration, psm string) *LongPool {
	limit := &CountLimiter{Max: maxIdleGlobal}
	dialer := &netDialer{}
	metrics := LongConnMetrics(psm)

	lp := &LongPool{
		newPeer: func(addr net.Addr) *peer {
			return newPeer(addr, maxIdlePerIns, maxIdleTimeout, limit, metrics, dialer)
		},
	}
	return lp
}
