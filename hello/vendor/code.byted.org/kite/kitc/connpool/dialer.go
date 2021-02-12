package connpool

import (
	"net"
	"time"
)

type netDialer struct{}

func (nd *netDialer) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout(network, address, timeout)
}
