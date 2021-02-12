package redis_aux

import (
	"fmt"

	"code.byted.org/gopkg/logs"
	"code.byted.org/kv/goredis"
	"git.byted.org/ttgame/platform/nvwa/conf"
	"git.byted.org/ttgame/platform/nvwa/environ"
)

/*
举例
下面的参数0表示默认值
redis_pool_size 5
redis_dial_timeout 1000ms
redis_read_timeout 200ms
redis_write_timeout 200ms
redis_pool_timeout 1000ms
redis_idle_timeout 10m
redis_live_timeout 1h
redis_max_retries 2
要连的PSM
redis_psm toutiao.redis.ttgame_platform_gops
redis_server 10.2.198.97:6379 #开发环境用
*/

const (
	MAX_RETRIES = 2
)

func NewClientWithConf() (*goredis.Client, error) {
	return NewClientWithConfPrefixKey("redis")
}

func makeKey(prefixKey, confKey string) string {
	return fmt.Sprintf("%s_%s", prefixKey, confKey)
}

func NewClientWithConfPrefixKey(prefixKey string) (*goredis.Client, error) {
	opt := goredis.NewOptionWithTimeout(
		conf.MustGetDuration(makeKey(prefixKey, "dial_timeout")),
		conf.MustGetDuration(makeKey(prefixKey, "read_timeout")),
		conf.MustGetDuration(makeKey(prefixKey, "write_timeout")),
		conf.MustGetDuration(makeKey(prefixKey, "pool_timeout")),
		conf.MustGetDuration(makeKey(prefixKey, "idle_timeout")),
		conf.MustGetDuration(makeKey(prefixKey, "live_timeout")),
		conf.MustGetInt(makeKey(prefixKey, "pool_size")),
	)

	var maxRetries int
	if conf.GetConf(makeKey(prefixKey, "max_retries")) == "" {
		maxRetries = MAX_RETRIES
	} else {
		maxRetries = conf.MustGetInt(makeKey(prefixKey, "max_retries"))
	}
	opt.SetMaxRetries(maxRetries)
	var (
		client *goredis.Client
		err    error
	)
	if environ.IsDevelop() {
		opt.DisableAutoLoadConf()
		client, err = goredis.NewClientWithServers(conf.GetConf(makeKey(prefixKey, "psm")), []string{conf.GetConf(makeKey(prefixKey, "server"))}, opt)
	} else {
		opt.SetServiceDiscoveryWithConsul()
		client, err = goredis.NewClientWithOption(conf.GetConf(makeKey(prefixKey, "psm")), opt)
	}
	if err != nil {
		logs.Fatalf("new redis client error, opt=%v, err=%v", opt, err)
		return nil, err
	}
	return client, nil
}
