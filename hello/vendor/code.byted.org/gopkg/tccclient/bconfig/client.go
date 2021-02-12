package bconfig

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"code.byted.org/gopkg/consul"
	"code.byted.org/gopkg/metrics"
	"golang.org/x/sync/singleflight"
)

var (
	hostIP = ""

	metricsClient *metrics.MetricsClientV2
)

func SetHostIP(ip string) {
	hostIP = ip
}

func init() {
	metricsClient = metrics.NewDefaultMetricsClientV2("bconfig.proxy", true)
}

type BConfigClient struct {
	oo      getoptions
	sf      singleflight.Group
	httpcli *http.Client
	Get     func(context.Context, string) (string, error)
}

type getoptions struct {
	cluster string
	addr    string
}

// GetOption represents option of get op
type GetOption func(o *getoptions)

// WithCluster sets cluster of get context
func WithCluster(cluster string) GetOption {
	return func(o *getoptions) {
		o.cluster = cluster
	}
}

// WithAddr sets addr for http request instead get from consul
func WithAddr(addr string) GetOption {
	return func(o *getoptions) {
		o.addr = addr
	}
}

// NewBConfigClient creates instance of BConfigClient
func NewBConfigClient(opts ...GetOption) *BConfigClient {
	oo := getoptions{cluster: "default"}
	for _, op := range opts {
		op(&oo)
	}
	c := &BConfigClient{oo: oo}

	dialer := net.Dialer{}
	c.httpcli = &http.Client{
		Transport: &http.Transport{
			DisableCompression: true,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				if oo.addr != "" {
					return dialer.DialContext(ctx, network, oo.addr)
				}

				ee, err := consul.LookupName("bytedance.bconfig.proxy")
				if err != nil {
					return nil, err
				}
				ee = ee.Filter(func(e consul.Endpoint) bool { return e.Cluster == c.oo.cluster })
				return dialer.DialContext(ctx, network, ee.GetOne().Addr)
			},
		},
	}
	c.Get = c.RealGet
	return c
}

// RealGet returns value specified by key
func (c *BConfigClient) RealGet(ctx context.Context, key string) (string, error) {
	ret, err, _ := c.sf.Do(key+"@"+c.oo.cluster, func() (interface{}, error) {
		s, err := c.get(ctx, key)
		return s, err
	})
	s := ret.(string)
	return s, err
}

func (c *BConfigClient) get(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", errors.New("key not specified")
	}
	uri := "http://bytedance.bconfig.proxy/v1/keys/" + strings.TrimLeft(key, "/")
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("X-Host-IP", hostIP)
	req = req.WithContext(ctx)
	t0 := time.Now()
	resp, err := c.httpcli.Do(req)
	c.emit(t0, err)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return decodeResponse(resp.Body)
}

func (c *BConfigClient) emit(t0 time.Time, err error) {
	status := "success"
	if err != nil {
		status = "failed"
	}
	cost := time.Since(t0).Nanoseconds() / 1000
	metricsClient.EmitCounter(fmt.Sprintf("get.%s.throughput", status), 1)
	metricsClient.EmitTimer(fmt.Sprintf("get.%s.latency", status), cost)
}
