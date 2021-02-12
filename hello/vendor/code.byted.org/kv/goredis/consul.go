package goredis

import (
	"net"
	"os"
	"strings"
	"sync"

	"code.byted.org/gopkg/consul"
	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/logs"
)

const (
	HY_IDC = "hy"
	LF_IDC = "lf"
)

var (
	localIDC   string
	localOncer sync.Once
)

// LocalIDC return idc's name of current service
// first read env val RUNTIME_IDC_NAME
func LocalIDC() string {
	localOncer.Do(func() {
		if dc := os.Getenv("RUNTIME_IDC_NAME"); dc != "" {
			localIDC = strings.TrimSpace(dc)
		} else {
			localIDC = env.IDC()
		}
	})
	return localIDC
}

type Instance struct {
	host string
	port string
	tags map[string]string
}

func NewInstance(host, port string, tags map[string]string) *Instance {
	for key, val := range tags {
		tags[key] = strings.TrimSpace(val)
	}
	return &Instance{
		host: strings.TrimSpace(host),
		port: strings.TrimSpace(port),
		tags: tags,
	}
}

func (it *Instance) Host() string {
	return it.host
}

func (it *Instance) Port() string {
	return it.port
}

func (it *Instance) Str() string {
	return net.JoinHostPort(it.host, it.port)
}

type ConsulService struct {
	name       string
	addrfamily string
}

func NewConsulService(name, family string) *ConsulService {
	logs.Info("addr family is useless, we will get from env BYTED_HOST_IP and BYTED_HOST_IPV6")
	return &ConsulService{name, family}
}

func (cs *ConsulService) Name() string {
	return cs.name
}

func (cs *ConsulService) AddrFamily() string {
	logs.Info("addr family is useless, we will get from env BYTED_HOST_IP and BYTED_HOST_IPV6")
	return ""
}

// Lookup return a list of instances
func (cs *ConsulService) Lookup(idc string) []*Instance {
	idc = strings.TrimSpace(idc)
	items, err := consul.LookupName(cs.name, consul.WithIDC(consul.IDC(idc)))
	if err != nil {
		logs.Errorf("Redisclient consul.Lookup cluster %s idc %s, error:%v", cs.name, idc, err)
		return nil
	}

	var ret []*Instance
	for _, ins := range items {
		host, port, err := net.SplitHostPort(ins.Addr)
		if err != nil {
			logs.Errorf("Redisclient net SplitHostPort addr %s, error:%v", ins.Addr, err)
			return nil
		}
		ret = append(ret, NewInstance(host, port, nil))
	}
	return ret
}
