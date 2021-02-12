package ginex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/pprof"
	"strconv"

	"github.com/kuangchanglang/graceful"

	"code.byted.org/gopkg/debug"
	"code.byted.org/gopkg/logs"
	"code.byted.org/hystrix/hystrix-go/hystrix"
	"code.byted.org/kite/kitc"
)

var (
	debugMux = http.NewServeMux()
)

// 业务代码注册debug handler
func RegisterDebugHandler(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	debugMux.HandleFunc(pattern, handler)
	debug.RegisterHandlerFunc(pattern, handler)
}

func RegisterDebugHTTPHandler(pattern string, handler http.Handler) {
	debugMux.Handle(pattern, handler)
	debug.RegisterHandler(pattern, handler)
}

func startDebugServer() {
	if !appConfig.EnablePprof {
		logs.Info("Debug server not enabled.")
		return
	}
	if appConfig.DebugPort == 0 {
		logs.Info("Debug port is not specified.")
		return
	}
	debug.SetEnable(appConfig.EnablePprof)
	debug.SetListenPort(strconv.Itoa(appConfig.DebugPort))
	initDebugHandler()
	logs.Infof("Start pprof and hystrix listen on: %d", appConfig.DebugPort)
	go debug.Run()
}

func initDebugHandler() {
	// pprof handler
	debugMux.HandleFunc("/debug/pprof/", pprof.Index)
	debugMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	debugMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	debugMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	debugMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	RegisterDebugHandler("/runtime/kitc", func(w http.ResponseWriter, r *http.Request) {
		info := kitc.DebugInfo()
		buf, _ := json.MarshalIndent(info, "", "  ")
		w.WriteHeader(200)
		w.Write(buf)
	})

	// hystrix handler
	hystrixStreamHandler := hystrix.NewStreamHandler()
	hystrixStreamHandler.Start()
	RegisterDebugHTTPHandler("/debug/hystrix.stream", hystrixStreamHandler)
}

func registerDebugServer(server *graceful.Server) {
	if !appConfig.EnablePprof {
		logs.Info("Debug server not enabled.")
		return
	}
	if appConfig.DebugPort == 0 {
		logs.Info("Debug port is not specified.")
		return
	}

	initDebugHandler()
	logs.Infof("Start pprof and hystrix listen on: %d", appConfig.DebugPort)
	server.Register(fmt.Sprintf("0.0.0.0:%d", appConfig.DebugPort), debugMux)
}
