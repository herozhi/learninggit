// +build go1.8,!go1.10
// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package net

import (
	"golang.org/x/sys/unix"
	"code.byted.org/gopkg/logs"
)

func DoReuseControl(network, address string, sysfd int) error {

	err := unix.SetsockoptInt(sysfd, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
	if err != nil {
		logs.Errorf("set socket option SO_REUSEADDR<Posix> error: %v", err)
		return err
	}

	err = unix.SetsockoptInt(sysfd, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
	if err != nil {
		logs.Errorf("set socket option SO_REUSEPORT<Posix> error: %v", err)
		return err
	}

	return nil
}
