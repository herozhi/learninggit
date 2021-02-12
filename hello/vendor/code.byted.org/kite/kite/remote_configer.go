package kite

import (
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"code.byted.org/gopkg/logs"
	"code.byted.org/kite/kitutil/cache"
	"code.byted.org/kite/kitutil/kvstore"
)

const ACLWhiteSupportKey = "/kite/acl/white_support"

var defaultRPCConfig = RPCConfig{
	ACLAllow: true,
}

type remoteConfiger struct {
	kiteServer      *RpcServer
	kvstorer        kvstore.KVStorer
	rpcConfCache    *cache.Asyncache // config about rpc
	serverConfCache *cache.Asyncache // config about this server
}

func newRemoteConfiger(kiteServer *RpcServer, kvstorer kvstore.KVStorer) *remoteConfiger {
	c := &remoteConfiger{
		kvstorer:   kvstorer,
		kiteServer: kiteServer,
	}
	c.rpcConfCache = cache.NewAsyncache(cache.Options{
		BlockIfFirst:    true,
		RefreshDuration: time.Second * 10,
		Fetcher:         c.fetchRemoteRPCConfig,
		ErrHandler:      c.errHandler,
		ChangeHandler:   nil,
		IsSame:          c.isSameRemoteRPCConfig,
	})
	c.serverConfCache = cache.NewAsyncache(cache.Options{
		RefreshDuration: time.Second * 10,
		Fetcher:         c.fetchLimit,
		ErrHandler:      c.errHandler,
		ChangeHandler:   nil,
		IsSame:          c.isSameLimit,
	})
	return c
}

// AllRemoteConfigs .
func (c *remoteConfiger) AllRemoteConfigs() map[string]interface{} {
	rpcConfs := c.rpcConfCache.Dump()
	srvConfs := c.serverConfCache.Dump()
	for k, v := range srvConfs {
		rpcConfs[k] = v
	}
	return rpcConfs
}

// GetQPSLimit .
func (c *remoteConfiger) GetQPSLimit() (int64, error) {
	key := overloadETCDPath("/kite/limit/qps")
	i := c.serverConfCache.Get(key, DefaultLimitQps)
	return i.(int64), nil
}

// GetConnLimit .
func (c *remoteConfiger) GetConnLimit() (int64, error) {
	key := overloadETCDPath("/kite/limit/conn")
	i := c.serverConfCache.Get(key, DefaultMaxConns)
	return i.(int64), nil
}

func (c *remoteConfiger) fetchLimit(key string) (interface{}, error) {
	var defaultVal string
	if strings.HasPrefix(key, "/kite/limit/conn") {
		defaultVal = fmt.Sprintf("%v", limitMaxConns)
	} else {
		defaultVal = fmt.Sprintf("%v", limitQPS)
	}
	val, err := c.kvstorer.GetOrCreate(key, defaultVal)
	if err != nil {
		return nil, err
	}
	lim, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid limit val, key=%s, val=%s", key, val)
	}
	return lim, nil
}

func (c *remoteConfiger) isSameLimit(key string, oldData, newData interface{}) bool {
	oldLim := oldData.(int64)
	newLim := newData.(int64)
	return oldLim == newLim
}

// GetRemoteConfig .
func (c *remoteConfiger) GetRemoteRPCConfig(r RPCMeta) (RPCConfig, error) {
	v := c.rpcConfCache.Get(r.String(), defaultRPCConfig)
	return v.(RPCConfig), nil
}

func (c *remoteConfiger) fetchRemoteRPCConfig(key string) (interface{}, error) {
	tmp := strings.Split(key, "/")
	if len(tmp) != 5 {
		return nil, fmt.Errorf("invalid remote key: %s", key)
	}
	r := RPCMeta{
		UpstreamService: tmp[0],
		UpstreamCluster: tmp[1],
		Service:         tmp[2],
		Cluster:         tmp[3],
		Method:          tmp[4],
	}

	acl, aclErr := c.getACL(r)
	stress, stressErr := c.getStressSwitch(r)

	err := aclErr
	if err == nil {
		err = stressErr
	}

	return RPCConfig{
		ACLAllow:        acl,
		StressBotSwitch: stress,
	}, err
}

func (c *remoteConfiger) getStressSwitch(r RPCMeta) (bool, error) {
	global := "/kite/stressbot/request/switch/global"
	val, globalErr := c.kvstorer.GetOrCreate(global, "off")
	if globalErr != nil {
		return false, globalErr
	}
	if val == "off" {
		return false, nil
	}
	if val != "on" {
		return false, fmt.Errorf("invalid global stress switch value: %v", val)
	}

	psmKey := fmt.Sprintf("/kite/stressbot/%s/%s/request/switch", r.Service, r.Cluster)
	val, psmErr := c.kvstorer.GetOrCreate(psmKey, "off")
	if psmErr != nil {
		return false, psmErr
	}
	if val == "off" {
		return false, nil
	} else if val == "on" {
		return true, nil
	}
	return false, fmt.Errorf("invalid psm stress switch value: %v", val)
}

// getACL : get access control list config
// Only when the return value is (false, nil), the request will be denied by ACL.
func (c *remoteConfiger) getACL(r RPCMeta) (bool, error) {
	key := path.Join("/kite/acl", confETCDPath(r))
	val, err := c.kvstorer.Get(key)
	if kvstore.IsKeyNotFound(err) {
		whiteSupportVal, _ := c.kvstorer.Get(ACLWhiteSupportKey)
		if whiteSupportVal == "1" {
			// if specific key is not found, and whiteSupportMode is open, look up service's ACL(white/black) mode.
			key := path.Join("/kite/acl", anyETCDPath(r))
			val, err = c.kvstorer.Get(key)
			if kvstore.IsKeyNotFound(err) {
				return true, nil
			}
		} else {
			// if specific key is not found, and whiteSupportMode is closed, do not deny.
			return true, nil
		}
	}

	if err != nil {
		return false, err
	}

	if val == "0" {
		return true, nil
	} else if val == "1" {
		return false, nil
	}
	return false, fmt.Errorf("invalid acl value: %s", val)
}

// endpointQPSLimit .
type endpointQPSLimit struct {
	Endpoint string `json:"endpoint"`
	QPSLimit int    `json:"qps_limit"`
}

// endpointQPSLimit .
type endpointQPSLimitList struct {
	Payload []endpointQPSLimit `json:"payload"`
}

// getEndpointQPSLimit .
func (c *remoteConfiger) getEndpointQPSLimit() (map[string]int, error) {
	limit := make(map[string]int)
	key := overloadETCDPath("/kite/limit/endpoint_qps")
	val, err := c.kvstorer.GetOrCreate(key, "{}")
	if err != nil {
		return limit, err
	}

	endpointLimitList := endpointQPSLimitList{}
	err = json.Unmarshal([]byte(val), &endpointLimitList)
	if err != nil {
		return limit, fmt.Errorf("get endpoint qps err: %v", err)
	}

	for _, l := range endpointLimitList.Payload {
		if l.QPSLimit > 0 {
			limit[l.Endpoint] = l.QPSLimit
		}
	}
	return limit, nil
}

func (c *remoteConfiger) isSameRemoteRPCConfig(key string, oldData, newData interface{}) bool {
	oldConf := oldData.(RPCConfig)
	newConf := newData.(RPCConfig)
	return oldConf.ACLAllow == newConf.ACLAllow
}

func (c *remoteConfiger) errHandler(key string, err error) {
	if kvstore.IsClientTimeout(err) || kvstore.IsKeyNotFound(err) {
		return
	}
	logs.Warnf("KITE: fetch remote config key: %s, err: %s", key, err.Error())
}

func confETCDPath(r RPCMeta) string {
	buf := make([]byte, 0, 100)
	buf = append(buf, r.UpstreamService...)
	buf = append(buf, '/')
	if r.UpstreamCluster != "default" && r.UpstreamCluster != "" {
		buf = append(buf, r.UpstreamCluster...)
		buf = append(buf, '/')
	}
	buf = append(buf, r.Service...)
	buf = append(buf, '/')
	if r.Cluster != "default" && r.Cluster != "" {
		buf = append(buf, r.Cluster...)
		buf = append(buf, '/')
	}
	buf = append(buf, r.Method...)
	return string(buf)
}

func anyETCDPath(r RPCMeta) string {
	buf := make([]byte, 0, 100)
	buf = append(buf, "any/any/"...)
	buf = append(buf, r.Service...)
	buf = append(buf, '/')
	if r.Cluster != "default" && r.Cluster != "" {
		buf = append(buf, r.Cluster...)
		buf = append(buf, '/')
	}
	buf = append(buf, r.Method...)
	return string(buf)
}

func overloadETCDPath(prefix string) string {
	p := path.Join(prefix, ServiceName)
	if ServiceCluster != "" && ServiceCluster != "default" {
		p = path.Join(p, ServiceCluster)
	}
	return p
}
