package kitc

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"code.byted.org/gopkg/metainfo"
	"code.byted.org/gopkg/thrift"
)

const (
	bufferedTransportLen = 8192
)

// Transport enchanced with thrift.TRichTransport
type Transport interface {
	io.ReadWriteCloser
	io.ByteReader
	io.ByteWriter
	Open() error
	IsOpen() bool
	Flush() (err error)
	WriteString(s string) (n int, err error)
	RemoteAddr() string
	OpenWithContext(ctx context.Context) error
}

func NewBufferedTransport(kc *KitcClient) Transport {
	return &BufferedTransport{client: kc}
}

// BufferedTransport implement thrift.TRichTransport
type BufferedTransport struct {
	*thrift.HeaderTransport
	ctx    context.Context
	conn   net.Conn
	client *KitcClient
}

// RemoteAddr
func (bt *BufferedTransport) RemoteAddr() string {
	// This method will be invoked right after network processing,
	// so we extract response headers here.
	for k, v := range bt.HeaderTransport.ReadHeaders() {
		metainfo.SetBackwardValue(bt.ctx, k, v)
	}

	if rip, ok := bt.HeaderTransport.ReadHeader(HeaderTransRemoteAddr); ok {
		return rip
	}
	if bt.conn != nil {
		return bt.conn.RemoteAddr().String()
	}
	return ""
}

// ToCluster
func (bt *BufferedTransport) ToCluster() string {
	if toCluster, ok := bt.HeaderTransport.ReadHeader(HeaderTransToCluster); ok {
		return toCluster
	}
	return ""
}

func (bt *BufferedTransport) OpenWithContext(ctx context.Context) error {
	rpcInfo := GetRPCInfo(ctx)
	conn := rpcInfo.Conn()
	if conn == nil {
		return errors.New("No target connection in the context")
	}
	bt.conn = conn
	bt.ctx = ctx

	spanTransportOpen(ctx, spanTagBufferedTransport)
	if ServiceMeshMode {
		timeout := getSocketTimeout(ctx) + meshMoreTimeout
		socket := thrift.NewTSocketFromConnTimeout(conn, timeout)
		trans := thrift.NewTBufferedTransport(socket, bufferedTransportLen)
		bt.HeaderTransport = thrift.NewHeaderTransport(trans)
		bt.HeaderTransport.SetClientType(thrift.TTHeaderUnframedClientType)
		bt.HeaderTransport.SetIntHeader(TRANSPORT_TYPE, "unframed")
		if intHeaders, ok := ctx.Value(THeaderInfoIntHeaders).(map[uint16]string); ok {
			for k, v := range intHeaders {
				bt.HeaderTransport.SetIntHeader(k, v)
			}
		}
		if headers, ok := ctx.Value(THeaderInfoHeaders).(map[string]string); ok {
			for k, v := range headers {
				bt.HeaderTransport.SetHeader(k, v)
			}
		}
	} else {
		timeout := getSocketTimeout(ctx)
		socket := thrift.NewTSocketFromConnTimeout(conn, timeout)
		trans := thrift.NewTBufferedTransport(socket, bufferedTransportLen)
		bt.HeaderTransport = thrift.NewHeaderTransport(trans)
		bt.HeaderTransport.SetClientType(thrift.UnframedDeprecated)
	}
	return nil
}

func (bt *BufferedTransport) Close() error {
	if bt.ctx != nil {
		spanTransportClose(bt.ctx)
	}
	return bt.HeaderTransport.Close()
}

func (bt *BufferedTransport) Flush() error {
	err := bt.HeaderTransport.Flush()
	if bt.ctx != nil {
		spanTransportFlush(bt.ctx)
	}
	return err
}

// NewFramedTransport return a FramedTransport
func NewFramedTransport(kc *KitcClient) Transport {
	return &FramedTransport{
		client:    kc,
		maxLength: kc.opts.MaxFramedSize,
	}
}

// FramedTransport implement thrift.TRichTransport
type FramedTransport struct {
	*thrift.HeaderTransport
	ctx       context.Context
	conn      net.Conn
	client    *KitcClient
	maxLength int32
}

func (ft *FramedTransport) RemoteAddr() string {
	// This method will be invoked right after network processing,
	// so we extract response headers here.
	for k, v := range ft.HeaderTransport.ReadHeaders() {
		metainfo.SetBackwardValue(ft.ctx, k, v)
	}

	if rip, ok := ft.HeaderTransport.ReadHeader(HeaderTransRemoteAddr); ok {
		return rip
	}
	if ft.conn != nil {
		return ft.conn.RemoteAddr().String()
	}
	return ""
}

// ToCluster
func (bt *FramedTransport) ToCluster() string {
	if toCluster, ok := bt.HeaderTransport.ReadHeader(HeaderTransToCluster); ok {
		return toCluster
	}
	return ""
}

// OpenWithContext connect a backend server acording the content of ctx
func (ft *FramedTransport) OpenWithContext(ctx context.Context) error {
	rpcInfo := GetRPCInfo(ctx)
	conn := rpcInfo.Conn()
	if conn == nil {
		return errors.New("No target connection in the context")
	}
	ft.conn = conn
	ft.ctx = ctx

	spanTransportOpen(ctx, spanTagFramedTransport)
	if ServiceMeshMode {
		timeout := getSocketTimeout(ctx) + meshMoreTimeout
		socket := thrift.NewTSocketFromConnTimeout(conn, timeout)
		trans := thrift.NewTBufferedTransport(socket, bufferedTransportLen)
		ft.HeaderTransport = thrift.NewHeaderTransport(trans)
		ft.HeaderTransport.SetClientType(thrift.TTHeaderFramedClientType)
		ft.HeaderTransport.SetIntHeader(TRANSPORT_TYPE, "framed")
		if intHeaders, ok := ctx.Value(THeaderInfoIntHeaders).(map[uint16]string); ok {
			for k, v := range intHeaders {
				ft.HeaderTransport.SetIntHeader(k, v)
			}
		}
		if headers, ok := ctx.Value(THeaderInfoHeaders).(map[string]string); ok {
			for k, v := range headers {
				ft.HeaderTransport.SetHeader(k, v)
			}
		}
	} else {
		timeout := getSocketTimeout(ctx)
		socket := thrift.NewTSocketFromConnTimeout(conn, timeout)
		trans := thrift.NewTBufferedTransport(socket, bufferedTransportLen)
		ft.HeaderTransport = thrift.NewHeaderTransport(trans)
		ft.HeaderTransport.SetClientType(thrift.FramedDeprecated)
	}

	return nil
}

func (ft *FramedTransport) Close() error {
	if ft.ctx != nil {
		spanTransportClose(ft.ctx)
	}
	return ft.HeaderTransport.Close()
}

func (ft *FramedTransport) Flush() error {
	err := ft.HeaderTransport.Flush()
	if ft.ctx != nil {
		spanTransportFlush(ft.ctx)
	}
	return err
}

func getSocketTimeout(ctx context.Context) time.Duration {
	// TODO(zhangyuanjia): use the max one between readtimeout and writetimeout ?
	rpcInfo := GetRPCInfo(ctx)
	timeout := time.Duration(rpcInfo.WriteTimeout) * time.Millisecond
	if rpcInfo.ReadTimeout > rpcInfo.WriteTimeout {
		timeout = time.Duration(rpcInfo.ReadTimeout) * time.Millisecond
	}

	deadline, ok := ctx.Deadline()
	if dur := deadline.Sub(time.Now()); ok && dur < timeout {
		timeout = dur
	}

	return timeout
}

func getMeshSocketTimeout(ctx context.Context, maxTimeout time.Duration) time.Duration {
	var timeout time.Duration
	if maxTimeout > 0 {
		timeout = maxTimeout
	} else {
		timeout = time.Duration(defaultMeshProxyConfig.WriteTimeout) * time.Millisecond
		if defaultMeshProxyConfig.ReadTimeout > defaultMeshProxyConfig.WriteTimeout {
			timeout = time.Duration(defaultMeshProxyConfig.ReadTimeout) * time.Millisecond
		}
	}

	deadline, ok := ctx.Deadline()
	if dur := deadline.Sub(time.Now()); ok && dur < timeout {
		timeout = dur
	}

	return timeout + meshMoreTimeout
}
