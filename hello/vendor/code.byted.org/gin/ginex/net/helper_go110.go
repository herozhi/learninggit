// +build go1.10

package net

import (
	"net"
	"net/http"
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

	if strings.HasPrefix(l.Addr().String(), "/") {
		return s.ServeTLS(l, certFile, keyFile)
	} else {
		return s.ServeTLS(ApplyKeepAlive(l), certFile, keyFile)
	}
}
