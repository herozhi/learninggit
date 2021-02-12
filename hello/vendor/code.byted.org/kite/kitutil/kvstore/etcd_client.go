package kvstore

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"code.byted.org/gopkg/etcdproxy"
)

const (
	cacheTime            = 10 * time.Second
	etcdProxyKiteCluster = "kite"
)

var (
	// defaultClient is created for easy use
	etcdProxyCli *etcdProxyClient
	once         sync.Once
)

// Client is etcdProxy client
type etcdProxyClient struct {
	etcdProxy *etcdproxy.EtcdProxy
}

// getEtcdProxyClient get etcdProxy client
func getEtcdProxyClient() *etcdProxyClient {
	once.Do(func() {
		etcdProxyCli = newEtcdProxyClient()
	})
	return etcdProxyCli
}

// NewClient create etcdProxy client
func newEtcdProxyClient() *etcdProxyClient {
	proxy := etcdproxy.NewEtcdProxy(
		etcdproxy.CacheTime(cacheTime),
		etcdproxy.WithCluster(etcdProxyKiteCluster),
	)
	return &etcdProxyClient{
		etcdProxy: proxy,
	}
}

func (c *etcdProxyClient) Get(ctx context.Context, key string) (val string, err error) {
	value, err := c.etcdProxy.Get(key)
	return value, convertErr(err)
}

const (
	errorCodeKeyNotFound = 100
)

type Error struct {
	Code    int    `json:"error_code"`
	Message string `json:"message"`
}

func (e Error) Error() string {
	return fmt.Sprintf("%v: %v", e.Code, e.Message)
}

func convertErr(err error) error {
	if err == nil {
		return nil
	}
	if etcdproxy.IsKeyNotFound(err) {
		// mock ErrKeyNotFound of etcd_client
		err = Error{
			Code:    errorCodeKeyNotFound,
			Message: "key not found",
		}
	}
	return err
}

func IsKeyNotFound(err error) bool {
	if cErr, ok := err.(Error); ok {
		return cErr.Code == errorCodeKeyNotFound
	}
	return false
}

func IsClientTimeout(err error) bool {
	if err != nil && strings.Contains(err.Error(), "Client.Timeout exceeded") {
		return true
	}
	return false
}
