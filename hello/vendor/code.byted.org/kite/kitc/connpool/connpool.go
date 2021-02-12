package connpool

import (
	"net"
	"time"
)

type Dialer interface {
	DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)
}

// ConnPool .
type ConnPool interface {
	// Get returns a connection to the given address.
	Get(network, address string, connTimeout time.Duration) (net.Conn, error)

	// Put puts the connection back to pool.
	// Note that the Close method of conn may already be invoked.
	Put(conn net.Conn) error

	// Discard discards the connection rather than putting it to the pool.
	Discard(conn net.Conn) error
}

type LongConnPool interface {
	ConnPool

	// Clean the state maintained in the poor for this address
	Clean(network, address string)
}
