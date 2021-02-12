package etcdutil

import (
	"context"
	"time"

	etcd "code.byted.org/gopkg/etcd_util/client"
	"code.byted.org/gopkg/etcdproxy"
	"code.byted.org/gopkg/logs"
	"code.byted.org/gopkg/metrics"
)

type Client struct {
	etcdProxy *etcdproxy.EtcdProxy
	aCache    *Asyncache
}

const (
	cacheTime = 0 * time.Second
)

// NewClient create client from ss_conf
func NewClient() (*Client, error) {
	var proxy *etcdproxy.EtcdProxy
	cacheTimeOpt := etcdproxy.CacheTime(cacheTime)
	if useAgent {
		addrOpt := etcdproxy.WithAddr(agentAddr)
		proxy = etcdproxy.NewEtcdProxy(cacheTimeOpt, addrOpt)
	} else {
		proxy = etcdproxy.NewEtcdProxy(cacheTimeOpt)
	}

	// Create Client
	c := &Client{
		etcdProxy: proxy,
	}

	aCache := NewAsyncache(Options{
		BlockIfFirst:    true,
		RefreshDuration: cacheRefreshInterval,
		Fetcher:         c.cacheFetch,
		ErrHandler: func(key string, err error) {
			if !etcd.IsKeyNotFound(err) {
				logs.Infof("ETCD: key=%s, err=%s", key, err.Error())
			}
		},
		IsSame: func(key string, oldData, newData interface{}) bool {
			return false
		},
		ChangeHandler: func(key string, oldData, newData interface{}) {
		},
	})
	c.aCache = aCache
	return c, nil
}

func (c *Client) cacheFetch(key string) (val interface{}, err error) {
	startTime := time.Now()
	defer func() {
		emitMetrics("get", startTime, err)
	}()

	value, err := c.etcdProxy.Get(key)
	if err != nil {
		err = convertErr(err)
		return nil, err
	}

	return value, nil
}

// Degrade, new client will use async_cache, async_cache will not return error
func (c *Client) Get(ctx context.Context, key string, opts *etcd.GetOptions) (resp *etcd.Response, err error) {
	startTime := time.Now()
	defer func() {
		emitMetrics("get", startTime, err)
	}()

	value, err := c.etcdProxy.Get(key)
	if err != nil {
		err = convertErr(err)
		return nil, err
	}

	etcdResp := etcd.Response{
		Node: &etcd.Node{
			Value: value,
		},
	}
	return &etcdResp, nil
}

func (c *Client) GetWithDefault(key string, defaultValue string) string {
	v := c.aCache.Get(key, defaultValue)
	return v.(string)
}

func convertErr(err error) error {
	if etcdproxy.IsKeyNotFound(err) {
		// mock ErrKeyNotFound of etcd_client
		err = etcd.Error{
			Code:    etcd.ErrorCodeKeyNotFound,
			Message: "key not found",
		}
	}
	return err
}

const metricsPrefix = "etcd.req"

var langTag = map[string]string{"lang": "go"}
var metricsClient = metrics.NewDefaultMetricsClient(metricsPrefix, true)

// emitMetrics upload metrics to server
func emitMetrics(methodName string, startTime time.Time, err error) {
	if err != nil {
		metricsClient.EmitCounter(methodName+".error.count", 1, metricsPrefix, langTag)
	}
	metricsClient.EmitCounter(methodName+".count", 1, metricsPrefix, langTag)
	metricsClient.EmitTimer(methodName+".latency", toMillisecond(time.Now())-toMillisecond(startTime), metricsPrefix, langTag)
}

// toMillisecond convert time to millisecond
// http://stackoverflow.com/a/24122933/1203241
func toMillisecond(input time.Time) int64 {
	return input.UnixNano() / int64(time.Millisecond)
}
