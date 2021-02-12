package pool

import (
	"net"
	"sync/atomic"
	"time"

	"code.byted.org/kv/redis-v6/pkg/proto"
)

var noDeadline = time.Time{}

type Conn struct {
	netConn net.Conn

	Rd *proto.Reader
	Wb *proto.WriteBuffer

	Inited   bool
	usedAt   atomic.Value
	createAt atomic.Value
}

func NewConn(netConn net.Conn) *Conn {
	cn := &Conn{
		netConn: netConn,
		Wb:      proto.NewWriteBuffer(),
	}
	cn.Rd = proto.NewReader(cn.netConn)
	cn.SetUsedAt(time.Now())
	cn.SetCreateAt(time.Now())
	return cn
}

func (cn *Conn) UsedAt() time.Time {
	return cn.usedAt.Load().(time.Time)
}

func (cn *Conn) CreateAt() time.Time {
	return cn.createAt.Load().(time.Time)
}

func (cn *Conn) SetUsedAt(tm time.Time) {
	cn.usedAt.Store(tm)
}

func (cn *Conn) SetCreateAt(tm time.Time) {
	cn.createAt.Store(tm)
}

func (cn *Conn) SetNetConn(netConn net.Conn) {
	cn.netConn = netConn
	cn.Rd.Reset(netConn)
}

func (cn *Conn) IsStale(timeout time.Duration, liveTimeout time.Duration) bool {
	if liveTimeout == 0 {
		return timeout > 0 && time.Since(cn.UsedAt()) > timeout
	}
	return (timeout > 0 && time.Since(cn.UsedAt()) > timeout) || (time.Since(cn.CreateAt()) > liveTimeout)
}

func (cn *Conn) SetReadTimeout(timeout time.Duration) error {
	now := time.Now()
	cn.SetUsedAt(now)
	if timeout > 0 {
		return cn.netConn.SetReadDeadline(now.Add(timeout))
	}
	return cn.netConn.SetReadDeadline(noDeadline)
}

func (cn *Conn) SetWriteTimeout(timeout time.Duration) error {
	now := time.Now()
	cn.SetUsedAt(now)
	if timeout > 0 {
		return cn.netConn.SetWriteDeadline(now.Add(timeout))
	}
	return cn.netConn.SetWriteDeadline(noDeadline)
}

func (cn *Conn) Write(b []byte) (int, error) {
	return cn.netConn.Write(b)
}

func (cn *Conn) RemoteAddr() net.Addr {
	return cn.netConn.RemoteAddr()
}

func (cn *Conn) LocalAddr() net.Addr {
	return cn.netConn.LocalAddr()
}

func (cn *Conn) Close() error {
	return cn.netConn.Close()
}
