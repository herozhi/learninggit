package net

import (
	"net"
	"time"
)

type keepAliveListener struct {
	*net.TCPListener
}

func (ln keepAliveListener) Accept() (net.Conn, error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}

func ApplyKeepAlive(l net.Listener) net.Listener {
	if tcpL, ok := l.(*net.TCPListener); ok {
		return keepAliveListener{tcpL}
	} else {
		return l
	}
}
