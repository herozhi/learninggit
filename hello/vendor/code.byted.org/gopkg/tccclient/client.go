package tccclient

import (
	"context"
	"fmt"
	"os"
	"time"

	etcdutil "code.byted.org/gopkg/etcd_util"
	etcd "code.byted.org/gopkg/etcd_util/client"
	"code.byted.org/gopkg/etcdproxy"
	"code.byted.org/gopkg/logs"
	"code.byted.org/gopkg/metrics"
)

const (
	ApiVersionV1     = "v1"
	TccEtcdKeyPrefix = "/tcc/" + ApiVersionV1 + "/"
)

type Client struct {
	serviceName string
	cluster     string
	env         string

	cache       *Cache
	parserCache *ParserCache

	disableMetrics bool
}

var (
	proxy     *etcdproxy.EtcdProxy
	needAgent bool
)

func initV1() {
	// TODO: make sure etcd_util supports fallback from agent to etcd proxy
	// etcdutil.SetFallbackStrategy()
	needAgent = checkNeedAgent()
	if !needAgent {
		proxy = etcdproxy.NewEtcdProxy()
	}
}

func checkNeedAgent() bool {
	confPath := "/etc/ss_conf"
	prodPath := "/opt/tiger/ss_conf/ss"

	p, err := os.Readlink(confPath)
	if err != nil {
		return true
	}

	if p != prodPath {
		return false
	}
	return true
}

// NewClient returns tcc client
func NewClient(serviceName string, config *Config) (*Client, error) {
	logs.Info("[tcc] TccClient is deprecated, please reference README and upgrade to TccClientV2")
	client := Client{}
	client.serviceName = serviceName
	client.cluster = config.Cluster
	client.env = config.Env
	client.cache = NewCache()
	client.parserCache = NewParserCache()
	client.disableMetrics = config.DisableMetrics

	_, err := etcdutil.GetDefaultClient()
	if err != nil {
		return nil, err
	}

	return &client, nil
}

func (c *Client) getRealKey(key string) string {
	return TccEtcdKeyPrefix + c.serviceName + "/" + c.cluster + "/" + key
}

func getDirectly(key string) (string, error) {
	value, err := proxy.Get(key)
	if err != nil {
		if etcdproxy.IsKeyNotFound(err) {
			return "", ConfigNotFoundError
		} else {
			return "", err
		}
	}
	return value, nil
}

// Get gets value by config key, may return error if the config doesn't exist
func (c *Client) Get(key string) (string, error) {
	if c.env == "prod" && !needAgent {
		return getDirectly(c.getRealKey(key))
	}
	value, _, err := c.getWithCache(key)
	return value, err
}

func (c *Client) getWithCache(key string) (string, bool, error) {
	item := c.cache.Get(key)
	if item != nil {
		if !item.Expired() {
			return item.Value, false, nil
		}
	}

	value, err := c.get(key)
	if err == nil {
		c.cache.Set(key, Item{Value: value, Expires: time.Now().Add(5 * time.Second)})
	} else if err != nil && err != ConfigNotFoundError && item != nil {
		c.cache.Set(key, Item{Value: item.Value, Expires: time.Now().Add(5 * time.Second)})
		return item.Value, false, nil
	}
	return value, true, err
}

func (c *Client) get(key string) (string, error) {
	client, err := etcdutil.GetDefaultClient()
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	resp, err := client.Get(ctx, c.getRealKey(key), nil)
	if err != nil {
		if etcd.IsKeyNotFound(err) {
			c.emit(nil, key)
			return "", ConfigNotFoundError
		}
		c.emit(err, key)
		return "", err
	}

	c.emit(nil, key)
	return resp.Node.Value, nil
}

func (c *Client) emit(err error, configKey string) {
	if c.disableMetrics {
		return
	}

	status := "success"
	code := 0
	if err != nil {
		status = "failed"
		code = 1
	}

	if (configKey == "opentracing_sampling" ||
		configKey == "whale_did_check_control" ||
		configKey == "whale_request_switch_percent" ||
		configKey == "whale_inaccuracy_valid_switch") &&
		status == "success" {
		// hack: do not record opentracing and whale's key, for checking upgrade of tcc_v2
		// delete this hack after 2019-12-31
		return
	}

	key := fmt.Sprintf("client.%s.get_config.%s", c.serviceName, status)
	ts := []metrics.T{
		metrics.T{Name: "cluster", Value: c.cluster},
		metrics.T{Name: "config_key", Value: configKey},
		metrics.T{Name: "env", Value: c.env},
		metrics.T{Name: "version", Value: clientVersion},
		metrics.T{Name: "api_version", Value: ApiVersionV1},
		metrics.T{Name: "language", Value: "go"},
		metrics.T{Name: "code", Value: fmt.Sprintf("%d", code)},
	}

	metricsClient.EmitCounter(key, 1, ts...)
}
