/*
 *  KITE RPC FRAMEWORK
 */

package kite

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"

	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/logs"
	"code.byted.org/gopkg/metrics"
	"code.byted.org/gopkg/thrift"
	"code.byted.org/kite/kitc/connpool"
	"code.byted.org/kite/kite/gls"
	"code.byted.org/kite/kitutil/kvstore"
)

// server metrics
const (
	errorServerThroughputFmt string = "service.thrift.%s.calledby.error.throughput"
)

type RpcServer struct {
	l net.Listener

	processorFactory thrift.TProcessorFactory
	transportFactory thrift.TTransportFactory
	protocolFactory  thrift.TProtocolFactory

	remoteConfiger *remoteConfiger
	overloader     *overloader
}

// NewRpcServer create the global server instance
func NewRpcServer() *RpcServer {
	// Using buffered transport and binary protocol as default,
	// buffer size is 4096
	var s *RpcServer
	oriTransport := thrift.NewTBufferedTransportFactory(DefaultTransportBufferedSize)
	transport := thrift.NewHeaderTransportFactory(oriTransport)
	protocol := thrift.NewHeaderProtocolFactory(thrift.ProtocolIDBinary)
	s = &RpcServer{
		transportFactory: transport,
		protocolFactory:  protocol,
	}

	// FIXME:when mesh mode, we don't need remoteConfiger&overloader,
	// but there are more functions depend with those,
	// so replace a empty kvstore with ETCD
	if ServiceMeshMode {
		s.remoteConfiger = newRemoteConfiger(s, newEmptyStorer())
		s.overloader = newOverloader(1<<20, 1<<20, limitQPSInterval)
	} else {
		s.remoteConfiger = newRemoteConfiger(s, kvstore.NewETCDStorer())
		s.overloader = newOverloader(limitMaxConns, limitQPS, limitQPSInterval)
	}
	return s
}

// CreateListener .
func (p *RpcServer) CreateListener() (net.Listener, error) {
	var addr string
	if ServiceMeshMode {
		addr = ServiceMeshIngressAddr
		if _, err := net.ResolveTCPAddr("tcp", addr); err == nil {
			ListenType = LISTEN_TYPE_TCP
		} else {
			ListenType = LISTEN_TYPE_UNIX
			syscall.Unlink(addr)
		}
	} else {
		if ListenType == LISTEN_TYPE_TCP {
			addr = net.JoinHostPort(ServiceAddr, ServicePort)
		} else if ListenType == LISTEN_TYPE_UNIX {
			addr = ServiceAddr
			syscall.Unlink(ServiceAddr)
		} else {
			return nil, errors.New(fmt.Sprintf("Invalid listen type %s", ListenType))
		}
	}

	l, err := net.Listen(ListenType, addr)
	if err != nil {
		return nil, err
	}
	if ListenType == LISTEN_TYPE_UNIX {
		os.Chmod(ServiceAddr, os.ModePerm)
	}
	return l, nil
}

// ListenAndServe ...
func (p *RpcServer) ListenAndServe() error {
	l, err := p.CreateListener()
	if err != nil {
		return err
	}
	return p.Serve(l)
}

// Serve ...
func (p *RpcServer) Serve(ln net.Listener) error {
	if p.l != nil {
		panic("KITE: Listener not nil")
	}
	p.l = ln
	logs.Info("KITE: server listening on %s", ln.Addr())

	if Processor == nil {
		panic("KITE: Processor is nil")
	}
	p.processorFactory = thrift.NewTProcessorFactory(Processor)

	p.startDaemonRoutines()

	for {
		// If l.Close() is called will return closed error
		conn, err := p.l.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return err
			}

			logs.Errorf("KITE: accept failed, err:%v", err)

			time.Sleep(10 * time.Millisecond) // too many open files ?
			continue
		}

		if !p.overloader.TakeConn() {
			msg := fmt.Sprintf("KITE: connection overload, limit=%v, now=%v, remote=%s\n",
				p.overloader.ConnLimit(), p.overloader.ConnNow(), conn.RemoteAddr().String())
			logs.Warnf(msg)
			p.onConnOverload()
			conn.Close()
			continue
		}

		go func(conn net.Conn) {
			defer func() {
				// panic recover
				if e := recover(); e != nil {
					if err, ok := e.(string); !ok || err != recoverMW {
						const size = 64 << 10
						buf := make([]byte, size)
						buf = buf[:runtime.Stack(buf, false)]
						logs.Fatal("KITE: panic in processor: %s: %s", e, buf)
					}
					p.onPanic()
				}
			}()
			// release overloader conn
			defer p.overloader.ReleaseConn()
			ctx, processor, transport, protocol := p.prepareRequests(conn)
			// close conn
			defer transport.Close()

			handleRPC := func() {
				if err := p.processRequests(ctx, processor, transport, protocol); err != nil {
					logs.Warnf("KITE: processing request error=%s, remote=%s", err, conn.RemoteAddr().String())
				}
			}

			if processorSupportContext {
				handleRPC()
			} else {
				if enableGLS || GetRealIP {
					// Use gls to pass context
					glsStorage := gls.NewStorage()
					defer func() {
						glsStorage.Clear() // break possible cycle references
					}()
					ctx = context.WithValue(ctx, glsStorageKey, glsStorage) // Keep a reference
					gls.SetGID(uint64(uintptr(glsStorage.ToUnsafePointer())), handleRPC)
				} else {
					handleRPC()
				}
			}
		}(conn)
	}
}

// Stop .
func (p *RpcServer) Stop() error {
	if p.l == nil {
		return nil
	}
	if err := p.l.Close(); err != nil {
		return err
	}
	deadline := time.After(ExitWaitTime)
	for {
		select {
		case <-deadline:
			return errors.New("deadline excceded")
		default:
			if p.overloader.ConnNow() == 0 {
				return nil
			}
			time.Sleep(time.Millisecond)
		}
	}
}

func (p *RpcServer) prepareRequests(conn net.Conn) (context.Context, thrift.TProcessor, thrift.TTransport, thrift.TProtocol) {
	ctx := context.Background()
	if GetRealIP {
		addr := conn.RemoteAddr().String()
		ctx = context.WithValue(ctx, addrFromConnection, addr)
	}
	conn2 := &connpool.ConnWithPkgSize{Conn: conn}

	client := thrift.NewTSocketFromConnTimeout(conn2, ReadWriteTimeout)
	processor := p.processorFactory.GetProcessor(client)
	transport := p.transportFactory.GetTransport(client)
	protocol := p.protocolFactory.GetProtocol(transport)

	if ServiceMeshMode {
		transport.(*thrift.HeaderTransport).SetClientType(thrift.TTHeaderUnframedClientType)
	} else {
		transport.(*thrift.HeaderTransport).SetClientType(thrift.UnframedBinaryDeprecated)
	}

	ctx = context.WithValue(ctx, infrastructionKey, protocol)

	protocol2 := &statProtocol{TProtocol: protocol}
	ctx = newCtxWithRPCStats(ctx, &rpcStats{conn: conn2, protocol: protocol2})
	return ctx, processor, transport, protocol2
}

func (p *RpcServer) processRequests(ctx context.Context, processor thrift.TProcessor, transport thrift.TTransport, protocol thrift.TProtocol) error {
	if !processorSupportContext && (enableGLS || GetRealIP) {
		glsStorage := ctx.Value(glsStorageKey).(gls.Storage)
		glsStorage.SetData("context", ctx) // Note: this will create a cycle reference
	}

	stats, _ := getRPCStats(ctx)

	// This loop for processing request on a connection.
	var ok bool
	var err error
	for {
		stats.preRequest()

		metricsClient.EmitCounter("kite.process.throughput", 1,
			metrics.T{"name", ServiceName},
			metrics.T{"cluster", env.Cluster()},
		)
		if !p.overloader.TakeQPS() {
			msg := "KITE: qps overload, close socket forcely"
			logs.Warnf(msg)
			p.onQPSOverload()
			return errors.New("KITE: qps overload, close socket forcely")
		}

		if processorSupportContext {
			ok, err = processor.(thrift.TProcessorWithContext).ProcessWithContext(ctx, protocol, protocol)
		} else {
			ok, err = processor.Process(protocol, protocol)
		}
		if err, ok := err.(thrift.TTransportException); ok {
			if err.TypeId() == thrift.END_OF_FILE ||
				// TODO(xiangchao.01): this timeout maybe not precision,
				// fix should in thrift package later.
				err.TypeId() == thrift.TIMED_OUT {
				return nil
			}
			if err.TypeId() == thrift.UNKNOWN_METHOD {
				name := fmt.Sprintf("toutiao.service.thrift.%s.process.error", ServiceName)
				metricsClient.EmitCounter(name, 1,
					metrics.T{"name", "UNKNOWN_METHOD"},
					metrics.T{"cluster", env.Cluster()},
				)
			}
			if opErr, ok := err.Err().(*net.OpError); ok {
				if strings.Contains(opErr.Error(), "broken pipe") {
					// Ignore write broken pipe error
					return nil
				}
			}
		}

		stats.postRequest()

		if err != nil {
			return err
		}
		if !ok {
			break
		}
	}
	return nil
}

func (p *RpcServer) getRPCConfig(r RPCMeta) RPCConfig {
	c, err := p.remoteConfiger.GetRemoteRPCConfig(r)
	if err != nil {
		logs.Warnf("KITE: get remote config for %v err: %s, default config will be used", r, err.Error())
		return defaultRPCConfig
	}
	return c
}

func (p *RpcServer) onPanic() {
	name := fmt.Sprintf("service.thrift.%s.panic", ServiceName)
	metricsClient.EmitCounter(name, 1, metrics.T{"name", ServiceName}, metrics.T{"cluster", env.Cluster()})
}

// RemoteConfigs .
func (p *RpcServer) RemoteConfigs() map[string]interface{} {
	return p.remoteConfiger.AllRemoteConfigs()
}

// Overload .
func (p *RpcServer) Overload() (connNow, connLim, qpsLim int64) {
	return p.overloader.ConnNow(), p.overloader.ConnLimit(), p.overloader.QPSLimit()
}

func (p *RpcServer) onListenFailed() {
	p.serverErrorMetrics("listen_failed")
}

func (p *RpcServer) onAcceptFailed() {
	p.serverErrorMetrics("accept_failed")
}

func (p *RpcServer) onConnOverload() {
	p.serverErrorMetrics("conn_overload")
}

func (p *RpcServer) onQPSOverload() {
	p.serverErrorMetrics("qps_overload")
}

func (p *RpcServer) serverErrorMetrics(errorType string) {
	name := fmt.Sprintf(errorServerThroughputFmt, ServiceName)
	metricsClient.EmitCounter(name, 1, metrics.T{"cluster", env.Cluster()}, metrics.T{"type", errorType})
}
