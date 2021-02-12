package goredis

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"code.byted.org/gopkg/logs"
	kext "code.byted.org/kv/goredis/trace/ext"
	redis "code.byted.org/kv/redis-v6"
	"code.byted.org/kv/redis-v6/pkg"
	"code.byted.org/kv/redis-v6/pkg/pool"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

type Client struct {
	*redis.Client

	cluster            string
	psm                string
	metricsServiceName string
	ctx                context.Context
	clusterpool        *MultiServPool /* cluster connpool */
}

// NewClient will create a new client with cluster name use the default timeout settings
func NewClient(cluster string) (*Client, error) {
	opt := NewOption()
	return NewClientWithOption(cluster, opt)
}

// NewClientWithOption will use user specified timeout settings in option
func NewClientWithOption(cluster string, opt *Option) (*Client, error) {
	servers, err := loadConfByClusterName(cluster, opt.configFilePath, opt.addrfamily, opt.useConsul)
	if err != nil {
		return nil, err
	}
	logs.Info("Cluster %v's server list is %v", cluster, servers)
	return NewClientWithServers(cluster, servers, opt)
}

// NewClientWithServers will create a new client with specified servers and timeout in option
func NewClientWithServers(cluster string, servers []string, opt *Option) (*Client, error) {
	if len(servers) == 0 {
		return nil, ErrEmptyServerList
	}
	if len(os.Getenv("SEC_KV_AUTH")) > 0 || isRedisCluster(cluster) {
		opt.GetRedisDpsToken = GetRedisDpsToken
		opt.VerifyRedisDpsReply = VerifyRedisDpsReply
	}
	serversCh := make(chan []string, 1)
	serversCh <- servers

	cli := &Client{
		Client:             redis.NewClient(opt.Options),
		cluster:            GetClusterName(cluster),
		psm:                checkPsm(),
		metricsServiceName: GetPSMClusterName(cluster),
		clusterpool:        NewMultiServPool(servers, serversCh, opt),
	}

	if opt.enableSendSdkInfo {
		infos := map[string]string{"psm": cluster}
		b, err := json.Marshal(infos)
		if err != nil {
			return nil, err
		}
		opt.Options.SdkInfo = string(b)
	}

	cli.WrapProcess(chainWrapProcessMiddleWares(cli))
	cli.WrapGetConn(cli.GetConn)
	cli.WrapReleaseConn(cli.ReleaseConn)

	//pre conn
	preidx := make([]int, opt.PoolInitSize)
	preconn := make([]*pool.Conn, opt.PoolInitSize)

	for i := range preidx {
		preconn[i], _, _ = cli.GetConn()
	}
	for _, cn := range preconn {
		if cn != nil {
			cli.ReleaseConn(cn, nil)
		}
	}
	isInWhiteList(cluster)

	if opt.autoLoadConf {
		autoLoadConf(cli.cluster, serversCh, opt)
	}

	return cli, nil
}

func (c *Client) clone() *Client {
	return &Client{
		Client:             c.Client,
		cluster:            c.cluster,
		psm:                c.psm,
		metricsServiceName: c.metricsServiceName,
		clusterpool:        c.clusterpool,
	}
}

// WithContext .
func (c *Client) WithContext(ctx context.Context) *Client {
	cc := c.clone()

	cc.ctx = ctx
	// pass ctx to redis-v6 client ctx
	cc.Client = cc.Client.WithContext(ctx)

	// wrap process should be placed after redis-v6 deep copy
	cc.WrapProcess(chainWrapProcessMiddleWares(cc))
	cc.WrapGetConn(cc.GetConn)
	cc.WrapReleaseConn(cc.ReleaseConn)

	return cc
}

func (c *Client) ClientContext() context.Context {
	if c.ctx != nil {
		return c.ctx
	}
	return context.Background()
}

/* get conn from multi servs pool */
func (c *Client) GetConn() (*pool.Conn, bool, error) {
	var span opentracing.Span
	if c.ctx != nil {
		span = opentracing.SpanFromContext(c.ctx)
	}
	if span != nil {
		span.LogFields(kext.EventKindConnectStart)
	}
	cn, isNew, server, err := c.clusterpool.getConnection()
	if err != nil {
		if pkg.IsBadConn(err, false) {
			server.servopt.breaker.Fail()
		} else {
			server.servopt.breaker.Success()
		}
		return nil, false, err
	}

	//need init
	if !cn.Inited {
		if err := c.Client.InitConn(cn); err != nil {
			server.servopt.breaker.Fail()
			cn.Inited = false
			_ = c.ReleaseConn(cn, err)
			return nil, false, err
		}
	}
	if span != nil {
		span.LogFields(kext.EventKindConnectEnd)
		ext.PeerAddress.Set(span, cn.RemoteAddr().String())
		kext.LocalAddress.Set(span, cn.LocalAddr().String())
		span.LogFields(kext.EventKindPkgSendStart)
	}
	return cn, isNew, nil
}

/* release conn to multi servs connpool, bad conn->remove done conn->put to connpool */
func (c *Client) ReleaseConn(cn *pool.Conn, err error) bool {
	return c.clusterpool.releaseConnection(cn, err)
}

func (c *Client) metricsWrapProcess(oldProcess func(cmd redis.Cmder) error) func(cmd redis.Cmder) error {
	return func(cmd redis.Cmder) error {
		// degradate
		/*
			if cmdDegredated(c.metricsServiceName, cmd.Name()) {
				cmd.SetErr(ErrDegradated)
				return ErrDegradated
			}
		*/
		// if stress rpc, hack args
		if prefix, ok := isStressTest(c.ctx); ok {
			isAbase := isAbaseCluster(c.Cluster())
			cmd = convertStressCMD(isAbase, prefix, cmd)
		}
		t0 := time.Now().UnixNano()
		err := oldProcess(cmd)
		latency := (time.Now().UnixNano() - t0) / 1000
		addCallMetrics(c.ctx, cmd.Name(), latency, err, c.cluster, c.psm, c.metricsServiceName, 1)

		return err
	}
}

func (c *Client) Pipeline() *Pipeline {
	pipe := c.NewPipeline("pipeline")
	return pipe
}

// this func will create a pipeline with name user specified
// the name will be used for pipeline metrics
func (c *Client) NewPipeline(pipelineName string) *Pipeline {
	pipe := &Pipeline{
		c.Client.Pipeline(),
		c,
		c.cluster,
		c.psm,
		c.metricsServiceName,
		pipelineName,
	}
	return pipe
}

func (c *Client) Cluster() string {
	return c.cluster
}

func (c *Client) PSM() string {
	return c.psm
}

func (c *Client) MetricsServiceName() string {
	return c.metricsServiceName
}
