// +build go1.8,!go1.10

package net

import (
	"net"
	"net/http"
	"code.byted.org/gopkg/logs"
	"strings"
)

func ListenAndServe(l net.Listener, s *http.Server) error {

	if strings.HasPrefix(l.Addr().String(), "/") {
		return s.Serve(l)
	} else {
		return s.Serve(ApplyKeepAlive(l))
	}
}

func ListenAndServeTLS(l net.Listener, s *http.Server, certFile, keyFile string) error {
	s.Addr = l.Addr().String()
	l.Close()

	logs.Warn("REUSEADDR option is dropped with TLS.")

	return s.ListenAndServeTLS(certFile, keyFile)
}
