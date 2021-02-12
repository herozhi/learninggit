package etcdutil

import (
	"sync"
	"time"

	"code.byted.org/gopkg/logs"
)

var cacheRefreshInterval = time.Second * 30

// defaultClient is created for easy use
var defaultClient *Client
var defaultClientMu sync.RWMutex

func GetDefaultClient() (*Client, error) {
	defaultClientMu.RLock()
	cli := defaultClient
	defaultClientMu.RUnlock()
	if cli != nil {
		return cli, nil
	}

	defaultClientMu.Lock()
	defer defaultClientMu.Unlock()
	cli = defaultClient
	if cli != nil {
		return cli, nil
	}
	var err error
	defaultClient, err = NewClient()
	return defaultClient, err
}

// Degrade, requestTimeout will not take effect because client will async refresh
func SetRequestTimeout(timeout time.Duration) {
	return
}

// SetCacheRefresh .
func SetCacheRefresh(interval time.Duration) {
	if interval < time.Second {
		interval = time.Second
	}
	cacheRefreshInterval = interval
	defaultClientMu.Lock()
	defer defaultClientMu.Unlock()
	defaultClient = nil
}

// Get NOTICE: the error returned always be nil
func Get(key string, defaultValue string) (string, error) {
	return GetWithDefault(key, defaultValue), nil
}

// GetWithDefault .
func GetWithDefault(key string, defaultValue string) string {
	cli, err := GetDefaultClient()
	if err != nil {
		logs.Infof("ETCD: create default client err: %v", err.Error())
		return defaultValue
	}
	return cli.GetWithDefault(key, defaultValue)
}
