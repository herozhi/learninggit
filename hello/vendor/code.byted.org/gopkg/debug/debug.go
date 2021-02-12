package debug

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"

	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/logs"

	"github.com/gorilla/mux"
)

const (
	defaultListenAddr = "0.0.0.0"
	defaultListenPort = "18888"

	// 注：这里的 addr 应当是完整地址，如"0.0.0.0:18888"、"[::]:18888"、"/absolute/path/to/socket.sock"
	// 与 TCE_DEBUG_PORT 不兼容，如果同时指定了 TCE_DEBUG_PORT 与 GOPKG_DEBUG_ADDR，会使用 GOPKG_DEBUG_ADDR
	debugAddrEnvKey = "GOPKG_DEBUG_ADDR"
)

// 是否开启 debug server，默认应当开启，在线上不访问不会有性能损耗
var enable = true

var (
	// debug 使用的日志函数
	logFunc func(v ...interface{})

	// 全局应当有且只有一个 debugServer，所以做成单例
	s *server

	// 保证只启动一次 s
	o sync.Once
)

func init() {
	logFunc = log.Print
	m := mux.NewRouter()
	s = &server{
		listenAddr: net.JoinHostPort(defaultListenAddr, defaultListenPort),
		s: &http.Server{
			Handler: m,
		},
		m:                 m,
		healthHandlerFunc: healthHandler,
	}
}

func RegisterHandlerFunc(path string, handlerFunc http.HandlerFunc) {
	s.m.HandleFunc(path, handlerFunc)
}

func RegisterHandler(path string, handler http.Handler) {
	s.m.Handle(path, handler)
}

func RegisterPath(path string) *mux.Router {
	return s.m.PathPrefix(path).Subrouter()
}

func SetNotFoundHandler(handlerFunc http.HandlerFunc) {
	s.m.NotFoundHandler = handlerFunc
}

func SetMethodNotAllowedHandler(handlerFunc http.HandlerFunc) {
	s.m.MethodNotAllowedHandler = handlerFunc
}

func SetHealthHandlerFunc(handlerFunc http.HandlerFunc) {
	s.healthHandlerFunc = handlerFunc
}

// 这里的 addr 应当是完整地址，如"0.0.0.0:18888"、"[::]:18888"、"/absolute/path/to/socket.sock"
func SetListenAddr(addr string) {
	s.listenAddr = addr
}

// Deprecated
// 仅为兼容性考虑保留
// 请使用 SetListenAddr
func SetListenPort(port string) {
	s.listenAddr = net.JoinHostPort(defaultListenAddr, port)
}

func SetEnable(e bool) {
	enable = e
}

// Run 真正启动 debug server，并且能保证仅启动一次，所以可以在 server、client 中 init 时均调用
func Run() {
	if !enable {
		logFunc("debug server is disabled")
		return
	}
	o.Do(func() {
		// 环境变量优先级最高
		if port := env.TCEDebugPort(); port != "" {
			s.listenAddr = net.JoinHostPort(defaultListenAddr, port)
		}
		if addr := os.Getenv(debugAddrEnvKey); addr != "" {
			s.listenAddr = addr
		}
		logFunc("starting debug server")
		s.run()
	})
}

// Shutdown gracefully shutdown debug server
func Shutdown(ctx context.Context) error {
	logFunc("shutting down debug server")
	return s.s.Shutdown(ctx)
}

// Close force close server
func Close() error {
	logFunc("closing debug server")
	return s.s.Close()
}

// SetLogger sets the log function with the given logger.
// This method is deprecated, please use SetLogFunc instead.
func SetLogger(l *logs.Logger) {
	logFunc = func(v ...interface{}) {
		s := fmt.Sprint(v...)
		l.Info("%s", s)
	}
}

// SetLogFunc sets the log function for debug.
// The debug package uses the log.Print from the standard library to output logs, by default.
func SetLogFunc(f func(v ...interface{})) {
	logFunc = f
}
