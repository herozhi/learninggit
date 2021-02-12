package ginex

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"code.byted.org/gin/ginex/configstorer"
	"code.byted.org/gin/ginex/internal"
	netex "code.byted.org/gin/ginex/net"
	renderex "code.byted.org/gin/ginex/render"
	"code.byted.org/gin/ginex/throttle"
	"code.byted.org/gopkg/logs"
	"code.byted.org/gopkg/stats"
	"code.byted.org/trace/trace-client-go"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/kuangchanglang/graceful"
)

type Engine struct {
	server *graceful.Server
	*gin.Engine
}

func New() *Engine {
	r := &Engine{
		Engine: gin.New(),
	}
	return r
}

// write to applog and gin.DefaultErrorWriter
type recoverWriter struct{}
type cleanHook func()

func (rw *recoverWriter) Write(p []byte) (int, error) {

	if appLogger != nil {
		appLogger.Error(string(p))
	}
	return gin.DefaultErrorWriter.Write(p)
}

//Default creates a gin Engine with following middlewares attached:
//  - Recovery
//  - Ctx
//  - Opentracing
//  - Access log
//  - Api metrics
//  - Throttle
func Default() *Engine {
	r := New()
	mwConfig := DefaultMiddlewareConfig()
	r.Use(mwConfig.MiddlewareList()...)
	return r
}

func (engine *Engine) Reload() error {
	if engine.server == nil {
		logs.Error("server is not running in graceful mode, Reload is not support")
		return nil
	}
	return engine.server.Reload()
}

// Run attaches the router to a http.Server and starts listening and serving HTTP requests.
// It also starts a pprof debug server and report framework meta info to bagent
func (engine *Engine) Run(addr ...string) (err error) {
	if len(addr) != 0 {
		logs.Warnf("Addr param will be ignored")
	}
	if err = Register(); err != nil {
		logs.Warnf("register fail, err is %s", err)
		return err
	}
	if listener, hook, err := createListener(); nil != err {
		logs.Warnf("create listen fail, err is %s", err)
		return err
	} else {

		errCh := make(chan error, 1)
		go func() {
			logs.Info("Run in %s mode", appConfig.Mode)
			server := &http.Server{Handler: engine}
			if err := doHttpServerConfig(server); nil != err {
				errCh <- err
			} else {
				errCh <- netex.ListenAndServe(listener, server)
			}
		}()
		startDebugServer()
		protocolExtra := generateProtocolExtra(engine)
		reportMetainfo(nil, protocolExtra)
		// start report go gc stats
		stats.DoReport(PSM())
		if err = waitSignal(errCh, hook); err != nil {
			logs.Warnf("wait signal fail, err is %s", err)
			return err
		}
		return nil
	}
}

func generateProtocolExtra(engine *Engine) *ProtocolExtra {
	maps := engine.generatePathPatternMappings()
	protocolExtra := &ProtocolExtra{
		PSM:      PSM(),
		Mappings: maps,
	}
	return protocolExtra
}

func (engine *Engine) generatePathPatternMappings() []Mapping {
	mappings := make([]Mapping, 0)
	for _, r := range engine.Routes() {
		m := Mapping{}
		m.Method = r.Method
		m.PathPattern = r.Path
		m.FunctionName = GetRealHandlerName(r.HandlerFunc, r.Handler)
		mappings = append(mappings, m)
	}
	return mappings
}

// RunTLSWithFileName attaches the router to a http.Server and starts listening and serving HTTPS (secure) requests.
// It is a shortcut for http.ListenAndServeTLS(addr, certFile, keyFile, router)
// Note: this method will block the calling goroutine indefinitely unless an error happens.
func (engine *Engine) RunTLSWithFileName(certFile, keyFile string) (err error) {
	if err = Register(); err != nil {
		return err
	}

	if listener, hook, err := createListener(); nil != err {

		return err
	} else {

		errCh := make(chan error, 1)
		go func() {
			logs.Info("Run in %s mode", appConfig.Mode)
			server := &http.Server{Handler: engine}
			if err := doHttpServerConfig(server); nil != err {
				errCh <- err
			} else {
				errCh <- netex.ListenAndServeTLS(listener, server, certFile, keyFile)
			}
		}()

		startDebugServer()
		protocolExtra := generateProtocolExtra(engine)
		reportMetainfo(map[string]string{"protocol": "https"}, protocolExtra)
		// start report go gc stats
		stats.DoReport(PSM())

		return waitSignal(errCh, hook)
	}
}

// RunTLSWithBlock attaches the router to a http.Server and starts listening and serving HTTPS (secure) requests.
// It is a shortcut for http.ListenAndServeTLS(addr, certFile, keyFile, router)
// Note: this method will block the calling goroutine indefinitely unless an error happens.
func (engine *Engine) RunTLSWithBlock(certBlock, keyBlock []byte) (err error) {
	if err = Register(); err != nil {
		return err
	}

	pair, err := tls.X509KeyPair(certBlock, keyBlock)
	if err != nil {
		return err
	}

	server := &http.Server{Handler: engine, TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}}}

	if listener, hook, err := createListener(); nil != err {

		return err
	} else {

		errCh := make(chan error, 1)
		go func() {
			logs.Info("Run in %s mode", appConfig.Mode)
			if err := doHttpServerConfig(server); nil != err {
				errCh <- err
			} else {
				// 不提供 certFile, keyFile 且有 Certificates 时，会使用 Certificates 中的证书
				errCh <- netex.ListenAndServeTLS(listener, server, "", "")
			}
		}()

		startDebugServer()
		protocolExtra := generateProtocolExtra(engine)
		reportMetainfo(map[string]string{"protocol": "https"}, protocolExtra)
		// start report go gc stats
		stats.DoReport(PSM())

		return waitSignal(errCh, hook)
	}
}

// RunGraceful support graceful reload with SIGHUP
//
// NOTE: platform windows unsupported, use Run instead
func (engine *Engine) RunGraceful(unixAddrs ...string) error {
	if graceful.IsMaster() {
		if err := Register(); err != nil {
			return err
		}
		defer StopRegister()
	}

	if graceful.IsWorker() {
		protocolExtra := generateProtocolExtra(engine)
		reportMetainfo(map[string]string{"protocol": "https"}, protocolExtra)
		// start report go gc stats
		stats.DoReport(PSM())
	}

	logs.Info("Run in %s mode", appConfig.Mode)
	engine.server = graceful.NewServer()
	engine.server.Register(fmt.Sprintf("0.0.0.0:%d", appConfig.ServicePort), engine)
	for _, addr := range unixAddrs {
		logs.Info("Adding unix socket addr: %v", addr)
		engine.server.RegisterUnix(addr, engine)
	}
	registerDebugServer(engine.server)

	return engine.server.Run()
}

// 解决handler被decorator修饰打印metrics的问题，See wraps.go wraps_test.go
func (engine *Engine) GETEX(relativePath string, handler gin.HandlerFunc, handlerName string) gin.IRoutes {
	internal.SetHandlerName(handler, handlerName)
	return engine.Engine.GET(relativePath, handler)
}

func (engine *Engine) POSTEX(relativePath string, handler gin.HandlerFunc, handlerName string) gin.IRoutes {
	internal.SetHandlerName(handler, handlerName)
	return engine.Engine.POST(relativePath, handler)
}

func (engine *Engine) PUTEX(relativePath string, handler gin.HandlerFunc, handlerName string) gin.IRoutes {
	internal.SetHandlerName(handler, handlerName)
	return engine.Engine.PUT(relativePath, handler)
}

func (engine *Engine) DELETEEX(relativePath string, handler gin.HandlerFunc, handlerName string) gin.IRoutes {
	internal.SetHandlerName(handler, handlerName)
	return engine.Engine.DELETE(relativePath, handler)
}

func (engine *Engine) AnyEX(relativePath string, handler gin.HandlerFunc, handlerName string) gin.IRoutes {
	internal.SetHandlerName(handler, handlerName)
	return engine.Engine.Any(relativePath, handler)
}

func (engine *Engine) HandleEX(httpMethod, relativePath string, handler gin.HandlerFunc, handlerName string) gin.IRoutes {
	internal.SetHandlerName(handler, handlerName)
	return engine.Engine.Handle(httpMethod, relativePath, handler)
}

// LoadHTMLRootAt recursively load html templates rooted at \templatesRoot
// eg. LoadHTMLRootAt("templates")
func (engine *Engine) LoadHTMLRootAt(templatesRoot string) {
	var files []string
	filepath.Walk(templatesRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logs.Error("Walk templates directory %s %v", templatesRoot, err)
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})

	if appConfig.HTMLAutoReload {
		durSec := appConfig.HTMLReloadIntervalSec
		if durSec <= 0 {
			durSec = 10
		}
		htmlDebug := render.HTMLDebug{Files: files, FuncMap: engine.FuncMap, Delims: render.Delims{Left: "{{", Right: "}}"}}
		engine.Engine.HTMLRender = &renderex.AutoReloadRender{Render: &htmlDebug, Dur: time.Duration(durSec * int(time.Second))}
	} else {
		engine.Engine.LoadHTMLFiles(files...)
	}
}

// 扩展Group方法，返回扩展RouterGroup
func (engine *Engine) GroupEX(relativePath string, handlers ...gin.HandlerFunc) *RouterGroup {
	return &RouterGroup{RouterGroup: engine.Group(relativePath, handlers...)}
}

func doHttpServerConfig(server *http.Server) error {
	if appConfig.DisableKeepAlive {
		logs.Info("HTTP Keep-Alive is Disabled.")
		server.SetKeepAlivesEnabled(false)
	}
	return nil
}

func createListener() (net.Listener, cleanHook, error) {
	var listener net.Listener
	var err error
	if appConfig.networkType == TCP {
		addr := fmt.Sprintf("%s:%d", appConfig.bindingAddress, appConfig.ServicePort)
		listener, err = netex.ListenWithConfig("tcp", addr, appConfig.addrPortReuse)
	} else if appConfig.networkType == DomainSocket {
		listener, err = netex.ListenWithConfig("unix", appConfig.domainSocketPath, false)
	} else {
		return nil, nil, errors.New(fmt.Sprintf("Unknown NetworkType %d", appConfig.networkType))
	}
	if nil != err {
		return nil, nil, err
	}

	logs.Infof("GINEX: http server listening on %s", listener.Addr())

	var ch cleanHook = func() {
		listener.Close()
		if appConfig.networkType == DomainSocket {
			// Unix sockets must be unlink()ed before being reused again.
			syscall.Unlink(appConfig.domainSocketPath)
		}
	}

	return listener, ch, err
}

func waitSignal(errCh <-chan error, hook cleanHook) error {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM)
	defer logs.Stop()
	defer StopRegister()
	defer trace.Close()

	for {
		select {
		case sig := <-ch:
			fmt.Printf("Got signal: %s, Exit..\n", sig)
			hook()
			return errors.New(sig.String())
		case err := <-errCh:
			fmt.Printf("Engine run error: %s, Exit..\n", err)
			return err
		}
	}
}

// Init inits ginex framework. It loads config options from yaml and flags, inits loggers and setup run mode.
// Ginex's other public apis should be called after Init.
func Init() {
	internalInit(nil)
}

// InitWithConfigGetter inits ginex framework with custom configStorer getter. It loads config options from yaml and flags, inits loggers and setup run mode.
func InitWithConfigGetter(fetcher configstorer.Fetcher) {
	internalInit(fetcher)
}

func internalInit(fetcher configstorer.Fetcher) {
	os.Setenv("GODEBUG", fmt.Sprintf("netdns=cgo,%s", os.Getenv("GODEBUG")))
	loadConf()
	initLog()
	initOpentracing()
	gin.SetMode(appConfig.Mode)

	// MY_CPU_LIMIT will be set as limit cpu cores.
	if v := os.Getenv("MY_CPU_LIMIT"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			runtime.GOMAXPROCS(n)
		}
	}

	if err := internal.InitConfigStorer(fetcher); err != nil {
		fmt.Printf("Init config storer err:%+v\n", err)
	}

	// refresh throttle limit from etcd on non-mesh mode
	if !appConfig.ServiceMeshMode {
		throttle.InitThrottle()
	}
}
