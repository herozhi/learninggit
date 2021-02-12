// +build go1.10
// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package net

import (
	"syscall"

	"golang.org/x/sys/unix"
	"code.byted.org/gopkg/logs"
)

func DoReuseControl(network, address string, c syscall.RawConn) error {
	var err error
	c.Control(func(fd uintptr) {
		err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
		if err != nil {
			logs.Errorf("set socket option SO_REUSEADDR<Posix> error: %v", err)
			return
		}

		err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
		if err != nil {
			logs.Errorf("set socket option SO_REUSEPORT<Posix> error: %v", err)
			return
		}
	})
	return err
}
