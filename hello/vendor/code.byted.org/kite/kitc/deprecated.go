package kitc

import (
	"strings"

	"code.byted.org/gopkg/env"
	"code.byted.org/kite/kitc/discovery"
)

/*
	下面这些接口都应该不建议是有, 仅仅留作兼容;
*/

const (
	ClusterKey = "cluster"
	EnvKey     = "env"
	IDCKey     = "idc"
)

type IKService interface {
	// Name return this service's name
	Name() string
	// Lookup return a list of service instance, conds like: dc, cluster, env
	Lookup(idc string) []*Instance
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

func (it *Instance) Tags() map[string]string {
	return it.tags
}

// Cluster return cluster name, if no cluster return "default"
func (it *Instance) Cluster() string {
	if it.tags == nil {
		return "default"
	}
	cluster, ok := it.tags[ClusterKey]
	if ok {
		return cluster
	}
	return "default"
}

// Env return env name, if no env return "prod"
func (it *Instance) Env() string {
	if it.tags == nil {
		return "prod"
	}
	env, ok := it.tags[EnvKey]
	if ok {
		return env
	}
	return "prod"
}

// customService implement IKService
type customService struct {
	name    string
	insList []*Instance
}

func (s *customService) Name() string {
	return s.name
}

// if tags == nil, that equal all idcs.
func (s *customService) Lookup(idc string) []*Instance {
	var ret []*Instance
	for _, ins := range s.insList {
		if _, ok := ins.tags[IDCKey]; !ok || ins.tags == nil || ins.tags[IDCKey] == idc {
			ret = append(ret, ins)
		}
	}
	return ret
}

func NewCustomIKService(name string, ins []*Instance) IKService {
	return &customService{
		name:    name,
		insList: ins,
	}
}

type ikserviceWrapper struct {
	ik IKService
}

// Discover .
func (ikw *ikserviceWrapper) Discover(serviceName, idc string) ([]*discovery.Instance, error) {
	ins := ikw.ik.Lookup(idc)
	dins := make([]*discovery.Instance, 0, len(ins))
	for _, i := range ins {
		dins = append(dins, discovery.NewInstance(i.Host(), i.Port(), i.Tags()))
	}

	return dins, nil
}

// LocalIDC .
func LocalIDC() string {
	return env.IDC()
}
