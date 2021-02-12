// +build go1.8,!go1.10

package net

import (
	"net"
	"reflect"
)

func ReuseControl(network, address string, l *net.TCPListener) error {

	v := reflect.ValueOf(l)
	fd := reflect.Indirect(v).FieldByName("fd")
	sysfdField := reflect.Indirect(fd).FieldByName("sysfd")
	sysfd := sysfdField.Int()

	DoReuseControl(network, address, int(sysfd))

	return nil
}
