package debug

import (
	"net"
	"net/http"
	_ "net/http/pprof" // for /debug/pprof

	"code.byted.org/gopkg/debug/internal/debug"
	hlog "code.byted.org/gopkg/debug/internal/log"
	"code.byted.org/gopkg/debug/internal/runtime"
	"github.com/gorilla/mux"
)

const (
	healthPath = "/health"
	pingPath   = "/ping"

	// DO NOT USE THESE IF YOU DON'T KNOW WHAT YOU ARE DOING!!!
	// 以下方法会对业务造成影响，请不要作为常规 / 监控手段使用！！！
	// 以下 API 可能随时被删除 / 更改，不提供任何兼容性保证！！！
	// 仅提供给 OnCall 同学使用！！！
	freeOSMemoryPath = "/internal/debug/freeOSMemory"
	buildInfoPath    = "/internal/debug/buildInfo"
	setGCPercentPath = "/internal/debug/setGCPercent"
	readGCStatsPath  = "/internal/debug/readGCStats"

	forceGCPath      = "/internal/runtime/forceGC"
	readMemStatsPath = "/internal/runtime/readMemStats"
	gomaxprocsPath   = "/internal/runtime/gomaxprocs"
	numCPUPath       = "/internal/runtime/numCPU"
	versionPath      = "/internal/runtime/version"

	setLevelPath = "/internal/log/setLevel"
)

var internalHandlerFunc = map[string]http.HandlerFunc{
	freeOSMemoryPath: debug.FreeOSMemoryHandler,
	buildInfoPath:    debug.BuildInfoHandler,
	setGCPercentPath: debug.SetGCPercentHandler,
	readGCStatsPath:  debug.ReadGCStatsHandler,

	forceGCPath:      runtime.ForceGCHandler,
	readMemStatsPath: runtime.ReadMemStatsHandler,
	gomaxprocsPath:   runtime.GOMAXPROCSHandler,
	numCPUPath:       runtime.NumCPUHandler,
	versionPath:      runtime.VersionHandler,

	setLevelPath: hlog.SetLevelHandler,
}

type server struct {
	listenAddr string

	listener net.Listener
	s        *http.Server
	m        *mux.Router

	healthHandlerFunc http.HandlerFunc
}

func (s *server) run() {
	var addr net.Addr
	var err error
	s.initHandler()
	addr, err = net.ResolveTCPAddr("tcp", s.listenAddr)
	if err != nil {
		addr, err = net.ResolveUnixAddr("unix", s.listenAddr)
		if err != nil {
			logFunc("unsupported debug address: %v", err)
			panic(err)
		}
	}

	s.listener, err = net.Listen(addr.Network(), addr.String())
	if err != nil {
		logFunc("create debug server listener failed: %v", err)
		panic(err)
	}

	logFunc("debug server listen at: ", s.listener.Addr())
	if err := s.s.Serve(s.listener); err != http.ErrServerClosed {
		logFunc("[ERR] server exited with: ", err)
		panic(err)
	}
}

func (s *server) initHandler() {
	s.m.HandleFunc(healthPath, s.healthHandlerFunc)
	s.m.HandleFunc(pingPath, pingHandler)

	for k, v := range internalHandlerFunc {
		RegisterHandlerFunc(k, v)
	}

	s.m.PathPrefix("/debug/pprof").Handler(http.DefaultServeMux)
}
