// +build go1.10

package net

import (
	"net"
	"code.byted.org/gopkg/logs"
)

func ReuseControl(network, address string, l *net.TCPListener) error {
	if conn, err := l.SyscallConn(); nil == err {
		if err := DoReuseControl(network, address, conn); nil == err {
			logs.Debug("socket reuse options applied.")
		} else {
			logs.Debug("socket reuse options failed to applied: %v", err)
		}
		return nil
	} else {
		return err
	}
}
