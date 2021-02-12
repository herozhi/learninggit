// +build go1.8,!go1.10

package net

import (
	"syscall"

	"golang.org/x/sys/windows"
	"code.byted.org/gopkg/logs"
)

func DoReuseControl(network, address string, sysfd int) error {

	err := windows.SetsockoptInt(windows.Handle(sysfd), windows.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	if nil != err {
		logs.Errorf("set socket option SO_REUSEADDR<Windows> error: %v", err)
		return err
	}

	return nil
}
