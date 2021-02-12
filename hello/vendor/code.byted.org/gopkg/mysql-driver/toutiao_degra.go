package mysql

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"strconv"

	"code.byted.org/gopkg/asyncache"
	"code.byted.org/gopkg/env"
	etcdutil "code.byted.org/gopkg/etcd_util"
	etcdclient "code.byted.org/gopkg/etcd_util/client"
	"code.byted.org/gopkg/logs"
)

var (
	msConfigPrefix string
	msCache        *asyncache.Asyncache
)

func init() {
	msConfigPrefix = fmt.Sprintf("/kite/switches/%s/", env.PSM())
	if env.Cluster() != "default" {
		msConfigPrefix += fmt.Sprintf("%s/", env.Cluster())
	}

	msCache = asyncache.NewAsyncache(asyncache.Options{
		BlockIfFirst:    true,
		RefreshDuration: time.Second * 10,
		Fetcher: func(key string) (interface{}, error) {
			cli, err := etcdutil.GetDefaultClient()
			if err != nil {
				return nil, err
			}
			v, err := cli.Get(context.Background(), key, nil)
			if err != nil {
				return nil, err
			}
			num, err := strconv.Atoi(v.Node.Value)
			return num, err
		},
		ErrHandler: func(key string, err error) {
			if err == nil || etcdclient.IsKeyNotFound(err) {
				return
			}
			logs.Errorf("[mysql-driver]: get etcd key: %s, err: %s", key, err)
		},
	})
}

func doDegradation(sql string, cfg *Config) bool {
	to := consulName2PSM(cfg.toutiaoConsulName)
	method, _ := getOperation(sql)
	key := buildKey(to, method)
	v := msCache.Get(key, 0)
	num := v.(int)
	if num == 0 {
		return false
	} else if num == 100 {
		return true
	}

	return defaultSafeRander.Intn(100) < num
}

func doDegradationNew(r MysqlReqMeta) bool {
	config := getRemoteConfig(r)
	if config.DegraPercent == 0 {
		return false
	} else if config.DegraPercent == 100 {
		return true
	}

	return defaultSafeRander.Intn(100) < config.DegraPercent
}

// /kite/config/from/fromCluster/to/toCluster/method
func buildKey(to, method string) string {
	buf := make([]byte, 0, 96)
	buf = append(buf, msConfigPrefix...)
	buf = append(buf, to...)
	buf = append(buf, '/')
	// toCluster must be "default", so ignore it
	buf = append(buf, method...)
	return string(buf)
}

// SafeRander is used for avoiding to use global's rand;
type SafeRander struct {
	pos     uint32
	randers [128]*rand.Rand
	locks   [128]*sync.Mutex
}

// NewSafeRander .
func NewSafeRander() *SafeRander {
	var randers [128]*rand.Rand
	var locks [128]*sync.Mutex
	for i := 0; i < 128; i++ {
		randers[i] = rand.New(rand.NewSource(time.Now().UnixNano()))
		locks[i] = new(sync.Mutex)
	}
	return &SafeRander{
		randers: randers,
		locks:   locks,
	}
}

// Intn .
func (sr *SafeRander) Intn(n int) int {
	x := atomic.AddUint32(&sr.pos, 1)
	x %= 128
	sr.locks[x].Lock()
	n = sr.randers[x].Intn(n)
	sr.locks[x].Unlock()
	return n
}

var defaultSafeRander = NewSafeRander()
