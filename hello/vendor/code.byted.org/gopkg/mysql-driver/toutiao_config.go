package mysql

import (
	"context"
	"code.byted.org/gopkg/asynccache"
	"time"
	"path"
	"fmt"
	"code.byted.org/gopkg/logs"
	etcdclient "code.byted.org/gopkg/etcd_util/client"
	"runtime"
	"code.byted.org/gopkg/etcd_util"
	"strconv"
	"strings"
)

const (
	BUF_SIZE = 64 << 10
)

var (
	configAsyncCache asynccache.AsyncCache
)

func init() {
	configAsyncCache = asynccache.NewAsyncCache(asynccache.Options{
		RefreshDuration: time.Second * 10,
		Fetcher:         fetchRemoteConfig,
		ErrorHandler: func(key string, err error) {
			if err == nil || etcdclient.IsKeyNotFound(err) {
				return
			}
			logs.Errorf("[mysql-driver] get etcd key: %s, err: %s", key, err)
		},
	})
}

type RemoteConfig struct {
	DegraPercent int

	// cb config
	CBIsOpen    bool
	CBErrRate   float64
	CBMinSample int
}

func getDefaultConfig() RemoteConfig {
	return RemoteConfig{
		DegraPercent: 0,
		CBIsOpen:     false,
		CBErrRate:    0.5,
		CBMinSample:  200,
	}
}

func getRemoteConfig(r MysqlReqMeta) RemoteConfig {
	val, _ := configAsyncCache.Get(r.String())
	if val != nil {
		if config, ok := val.(RemoteConfig); ok {
			return config
		}
	}
	return getDefaultConfig()
}

func fetchRemoteConfig(key string) (result interface{}, e error) {
	defer func() {
		if e := recover(); e != nil {
			buf := make([]byte, BUF_SIZE)
			buf = buf[:runtime.Stack(buf, false)]
			logs.Fatal("[mysql-driver] toutiao_config panic: error=%+v stack=%s", e, buf)
		}
	}()
	r, err := getMysqlRPCMetaByStr(key)
	if err != nil {
		return nil, err
	}
	config := getDefaultConfig()
	if len(r.From) == 0 || len(r.To) == 0 || len(r.Method) == 0 {
		return config, nil
	}
	config.DegraPercent, err = getDegraPercent(r)
	if err != nil {
		logs.Errorf("[mysql-driver] fetch remote config degra percent error: %v", err)
		return nil, err
	}
	config.CBIsOpen, err = getServiceCBSwitch(r)
	if err != nil {
		logs.Errorf("[mysql-driver] fetch remote config service switch error: %v", err)
		return nil, err
	}
	config.CBMinSample, err = getServiceCBMinSample(r)
	if err != nil {
		logs.Errorf("[mysql-driver] fetch remote config cb min sample error: %v", err)
		return nil, err
	}
	config.CBErrRate, err = getServiceCBErrRate(r)
	if err != nil {
		logs.Errorf("[mysql-driver] fetch remote config cb err rate error: %v", err)
		return nil, err
	}
	return config, nil
}

func getDegraPercent(r MysqlReqMeta) (int, error) {
	key := path.Join("/kite/switches", confETCDPath(r))
	val, err := etcdutil.Get(key, "0")
	if err != nil {
		return 0, err
	}

	per, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("invalid degradation percent value: %s", val)
	}
	return per, err
}

func getServiceCBSwitch(r MysqlReqMeta) (bool, error) {
	key := path.Join("/kite/circuitbreaker/switch", confETCDPath(r))
	val, err := etcdutil.Get(key, "0")
	if err != nil {
		return false, err
	}

	if val == "1" {
		return true, nil
	} else if val == "0" {
		return false, nil
	}

	return false, fmt.Errorf("invalid circuitbreaker switch value: %s", val)
}

func getServiceCBErrRate(r MysqlReqMeta) (float64, error) {
	key := path.Join("/kite/circuitbreaker/config", confETCDPath(r), "errRate")
	val, err := etcdutil.Get(key, "0.5")
	if err != nil {
		return 0, err
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid circuitbreaker error rate value: %s", val)
	}

	return f, nil
}

func getServiceCBMinSample(r MysqlReqMeta) (int, error) {
	key := path.Join("/kite/circuitbreaker/config", confETCDPath(r), "minSample")
	val, err := etcdutil.Get(key, "200")
	if err != nil {
		return 0, err
	}
	num, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("invalid circuitbreaker min sample value: %s", val)
	}

	return num, nil
}

func confETCDPath(r MysqlReqMeta) string {
	buf := make([]byte, 0, 100)
	buf = append(buf, r.From...)
	buf = append(buf, '/')
	if r.FromCluster != "default" && r.FromCluster != "" {
		buf = append(buf, r.FromCluster...)
		buf = append(buf, '/')
	}
	buf = append(buf, r.To...)
	buf = append(buf, '/')
	if r.ToCluster != "default" && r.ToCluster != "" {
		buf = append(buf, r.ToCluster...)
		buf = append(buf, '/')
	}
	buf = append(buf, r.Method...)
	return string(buf)
}

type MysqlReqMeta struct {
	From        string
	FromCluster string
	To          string
	ToCluster   string
	Method      string
	Table       string
}

func (r MysqlReqMeta) String() string {
	sum := len(r.From) + len(r.FromCluster) + len(r.To) + len(r.ToCluster) + len(r.Method) + 4
	sb := strings.Builder{}
	sb.Grow(sum)
	sb.WriteString(r.From)
	sb.WriteByte('/')
	sb.WriteString(r.FromCluster)
	sb.WriteByte('/')
	sb.WriteString(r.To)
	sb.WriteByte('/')
	sb.WriteString(r.ToCluster)
	sb.WriteByte('/')
	sb.WriteString(r.Method)
	return sb.String()
}

func getMysqlRPCMetaByStr(metaStr string) (MysqlReqMeta, error) {
	result := MysqlReqMeta{}
	tmp := strings.Split(metaStr, "/")
	if len(tmp) != 5 {
		return result, fmt.Errorf("invalid RPC config key: %s", metaStr)
	}
	result.From = tmp[0]
	result.FromCluster = tmp[1]
	result.To = tmp[2]
	result.ToCluster = tmp[3]
	result.Method = tmp[4]
	return result, nil
}

func getMysqlRPCMeta(ctx context.Context, cfg *Config, sql string) MysqlReqMeta {
	operation, _ := getOperation(sql)
	r := MysqlReqMeta{
		From:        serviceName,
		FromCluster: serviceCluster,
		To:          cfg.toutiaoConsulName,
		ToCluster:   "default",
		Method:      operation,
		Table:       getTableName(operation, sql),
	}
	return r
}
