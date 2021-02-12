// +build go1.10

package net

import (
	"syscall"

	"golang.org/x/sys/windows"
	"code.byted.org/gopkg/logs"
)

func DoReuseControl(network, address string, c syscall.RawConn) (err error) {
	return c.Control(func(fd uintptr) {
		err = windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
		if nil != err {
			logs.Errorf("set socket option SO_REUSEADDR<Windows> error: %v", err)
		}
	})
}
