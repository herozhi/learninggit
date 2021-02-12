package discovery

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"code.byted.org/gopkg/consul"
	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/etcdproxy"
	"code.byted.org/gopkg/logs"
)

var DegradedIPPolicy = IPV4Only

var ipPolicyEtcdKey = func() string {
	if env.Cluster() == env.ClusterDefault {
		return fmt.Sprintf("/kite/priorityaddr/%s/*/*/*/priority", env.PSM())
	}
	return fmt.Sprintf("/kite/priorityaddr/%s/%s/*/*/*/priority", env.PSM(), env.Cluster())
}()

func isIPV4Only(degradedIPPolicy IPPolicy) bool {
	priority, err := etcdproxy.NewEtcdProxy(etcdproxy.WithCluster("kite")).Get(ipPolicyEtcdKey)
	if err != nil {
		if etcdproxy.IsKeyNotFound(err) {
			return true
		}
		// must success https://bytedance.feishu.cn/docs/doccn0nJpeHBEbgOIjJLeZLGhEf
		// or degrade from config
		logs.Warn("get ip stack policy from etcd key: %s, error: %v, degrade: %v", ipPolicyEtcdKey, err, degradedIPPolicy == IPV4Only)
		return degradedIPPolicy == IPV4Only
	}
	return priority == "ipv4"
}

type IPPolicy int

const (
	IPV4Only IPPolicy = iota
	IPV6Priority
)

var (
	lookup         func(psm, idc string) (consul.Endpoints, error)
	initLookupOnce sync.Once
)

func initLookup() {
	isIPV4Only := isIPV4Only(DegradedIPPolicy)
	var lookupOptions []consul.LookupOptions
	switch {
	case env.IsIPV4Only():
	case env.IsIPV6Only():
		if isIPV4Only {
			lookup = func(psm, idc string) (consul.Endpoints, error) {
				logs.Error("self ipv6only but use ipv4only policy")
				return nil, consul.ErrNoEndpoint
			}
			return
		}
		lookupOptions = append(lookupOptions, consul.WithAddrFamily(consul.V6))
	case env.IsDualStack():
		if !isIPV4Only {
			lookupOptions = append(lookupOptions, consul.WithAddrFamily(consul.DUALSTACK), consul.WithUnique(consul.V6))
		}
	}
	lookup = func(psm, idc string) (consul.Endpoints, error) {
		idc = strings.TrimSpace(idc)
		opts := append(lookupOptions, consul.WithIDC(consul.IDC(idc)))
		return consul.Lookup(psm, opts...)
	}
}

// ConsulDiscover discover this service with specifical idc by consul
func ConsulDiscover(serviceName, idc string) ([]*Instance, error) {
	initLookupOnce.Do(func() {
		initLookup()
	})
	items, err := lookup(serviceName, idc)
	if err != nil {
		return nil, err
	}

	var ret []*Instance
	for _, ins := range items {
		host, port, err := net.SplitHostPort(ins.Addr)
		if err != nil {
			return nil, err
		}
		tags := make(map[string]string, len(ins.Tags))
		for k, v := range ins.Tags {
			tags[k] = v
		}
		tags["weight"] = strconv.Itoa(ins.Weight)
		ret = append(ret, NewInstance(host, port, tags))
	}
	return ret, nil
}

// ConsulDiscoverer .
type ConsulDiscoverer struct{}

// NewConsulDiscoverer .
func NewConsulDiscoverer() *ConsulDiscoverer {
	return &ConsulDiscoverer{}
}

// Discover .
func (c *ConsulDiscoverer) Discover(serviceName, idc string) ([]*Instance, error) {
	return ConsulDiscover(serviceName, idc)
}
