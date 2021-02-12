package kite

/*
   提供端口供pprof
*/

import (
	"encoding/json"
	"net/http"

	"code.byted.org/gopkg/debug"
	"code.byted.org/gopkg/logs"
	"code.byted.org/kite/kitc"
)

var healthHandler = func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

// RegisterHealthCheck
func RegisterHealthCheck(handler func(http.ResponseWriter, *http.Request)) {
	healthHandler = handler
}

// RegisterDebugHandler add custom http interface and handler
func RegisterDebugHandler(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	debug.RegisterHandlerFunc(pattern, handler)
}

func startDebugServer() {
	if !EnableDebugServer {
		logs.Info("KITE: Debug server not enabled.")
		return
	}
	debug.SetEnable(EnableDebugServer)
	RegisterDebugHandler("/version", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(ServiceVersion))
	})
	RegisterDebugHandler("/runtime/kite", runtimeKite)
	RegisterDebugHandler("/runtime/kitc", runtimeKitc)
	RegisterDebugHandler("/idl/kite", idlKite)
	RegisterDebugHandler("/idl/kitc", idlKitc)
	RegisterDebugHandler("/healthcheck", healthHandler)

	logs.Info("KITE: Start pprof listen on: %s", DebugServerPort)
	debug.SetListenPort(DebugServerPort)
	go debug.Run()
}

func runtimeKite(w http.ResponseWriter, r *http.Request) {
	confs := RPCServer.RemoteConfigs()
	connNow, connLim, qpsLim := RPCServer.Overload()
	meta := RPCServer.Metainfo()

	m := map[string]interface{}{
		"remote_configs": confs,
		"metainfo":       meta,
		"overload": map[string]interface{}{
			"conn_now": connNow,
			"conn_lim": connLim,
			"qps_lim":  qpsLim,
		},
	}
	buf, _ := json.MarshalIndent(m, "", "  ")
	w.WriteHeader(200)
	w.Write(buf)
}

func runtimeKitc(w http.ResponseWriter, r *http.Request) {
	info := kitc.DebugInfo()
	buf, _ := json.MarshalIndent(info, "", "  ")
	w.WriteHeader(200)
	w.Write(buf)
}

func idlKite(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	for fname, content := range IDLs {
		w.Write([]byte("==== " + fname + ":\n"))
		w.Write([]byte(content))
	}
}

func idlKitc(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	for service, m := range kitc.IDLs {
		for fname, content := range m {
			w.Write([]byte("==== " + service + " " + fname + ":\n"))
			w.Write([]byte(content))
		}
	}
}
