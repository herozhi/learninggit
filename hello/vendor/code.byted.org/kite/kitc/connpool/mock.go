package connpool

import (
	"net"
	"time"
)

var (
	_ net.Conn     = &mockConn{}
	_ Dialer       = &mockDialer{}
	_ ConnPool     = &DummyPool{}
	_ LongConnPool = &DummyPool{}
	_ ConnMetrics  = DummyConnMetrics("")
)

// mockConn mocks a net.Conn with behaviors configurable
type mockConn struct {
	Reader func(b []byte) (n int, err error)
	Writer func(b []byte) (n int, err error)
	Closer func() error
	Local  net.Addr
	Remote net.Addr
}

func (mc *mockConn) Read(b []byte) (n int, err error) {
	if mc.Reader != nil {
		return mc.Reader(b)
	}
	return 0, nil
}

func (mc *mockConn) Write(b []byte) (n int, err error) {
	if mc.Writer != nil {
		return mc.Writer(b)
	}
	return 0, nil
}

func (mc *mockConn) Close() error {
	if mc.Closer != nil {
		return mc.Closer()
	}
	return nil
}

func (mc *mockConn) LocalAddr() net.Addr                { return mc.Local }
func (mc *mockConn) RemoteAddr() net.Addr               { return mc.Remote }
func (mc *mockConn) SetDeadline(t time.Time) error      { return nil }
func (mc *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (mc *mockConn) SetWriteDeadline(t time.Time) error { return nil }

type mockDialer struct {
	DialFunc func(network string, address string, timeout time.Duration) (net.Conn, error)
}

func (md *mockDialer) DialTimeout(network string, address string, timeout time.Duration) (net.Conn, error) {
	return md.DialFunc(network, address, timeout)
}

type DummyPool struct{}

func (p *DummyPool) Get(network, address string, connTimeout time.Duration) (net.Conn, error) {
	return nil, nil
}
func (p *DummyPool) Put(conn net.Conn) error       { return nil }
func (p *DummyPool) Discard(conn net.Conn) error   { return nil }
func (p *DummyPool) Clean(network, address string) {}

type DummyConnMetrics string

func (dcm DummyConnMetrics) ConnSucc(addr net.Addr)  {}
func (dcm DummyConnMetrics) ConnFail(addr net.Addr)  {}
func (dcm DummyConnMetrics) ReuseSucc(addr net.Addr) {}
