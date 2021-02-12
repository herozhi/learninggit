package net

import (
	"net"
	"code.byted.org/gopkg/logs"
)

func ListenWithConfig(network, address string, addrPortReuse bool) (net.Listener, error) {
	if "tcp" == network {
		if l, err := net.Listen(network, address); nil == err {
			if tl, ok := l.(*net.TCPListener); addrPortReuse && ok {
				ReuseControl(network, address, tl)
				return tl, nil
			} else if !ok {
				logs.Error("cast to *net.TCPListener failure.")
			}
			return l, err
		} else {
			return nil, err
		}
	} else {
		return net.Listen(network, address)
	}
}
