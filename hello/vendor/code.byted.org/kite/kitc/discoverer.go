package kitc

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"code.byted.org/gopkg/logs"
	"code.byted.org/kite/endpoint"
	"code.byted.org/kite/kitc/discovery"
	"code.byted.org/kite/kitutil"
	"code.byted.org/kite/kitutil/cache"
	"code.byted.org/kite/kitutil/kiterrno"
	"code.byted.org/kite/kitutil/kitevent"
)

func makeFetchKey(service, idc, cluster, env string) string {
	return service + ":" + idc + ":" + cluster + ":" + env
}

// kitcDiscoverer 根据tag信息对discover发现的实例进行过滤, 并进行缓存
type kitcDiscoverer struct {
	eventBus kitevent.EventBus
	discover discovery.ServiceDiscoverer
	cache    *cache.Asyncache
	policy   *discovery.DiscoveryPolicy
}

func newKitcDiscoverer(ebus kitevent.EventBus, insp kitutil.Inspector, discover discovery.ServiceDiscoverer) *kitcDiscoverer {
	d := &kitcDiscoverer{
		discover: discover,
		eventBus: ebus,
	}
	c := cache.NewAsyncache(cache.Options{
		BlockIfFirst:    true,
		RefreshDuration: time.Second * 3,
		Fetcher:         d.fetch,
		ErrHandler:      d.fetchErrHandlerfunc,
		ChangeHandler:   d.instancesChangeHandler,
		IsSame:          d.isSameInstances,
	})
	d.cache = c

	d.eventBus.Watch(REMOTE_CONFIG_IDC_REMOVE, func(event *kitevent.KitEvent) {
		d.cache.DelPrefix(event.Detail)
	})

	insp.Register(func(data map[string]interface{}) {
		data["downstream_instances"] = d.Dump()
	})
	return d
}

func (d *kitcDiscoverer) Discover(serviceName, idc, cluster, env string) ([]*discovery.Instance, error) {
	key := makeFetchKey(serviceName, idc, cluster, env)
	v := d.cache.Get(key, []*discovery.Instance{})
	ins := v.([]*discovery.Instance)

	d.eventBus.Dispatch(&kitevent.KitEvent{
		Name:   SERVICE_DISCOVERY_SUCCESS,
		Time:   time.Now(),
		Detail: key,
		Extra: map[string]interface{}{
			"key": key,
			"new": ins,
		},
	})

	if len(ins) == 0 {
		return nil, fmt.Errorf("no instance for service: %s, idc: %s, cluster: %s, env: %s",
			serviceName, idc, cluster, env)
	}

	copied := make([]*discovery.Instance, len(ins))
	copy(copied, ins)
	return copied, nil
}

func (d *kitcDiscoverer) Dump() map[string][]*discovery.Instance {
	data := d.cache.Dump()
	result := make(map[string][]*discovery.Instance)
	for k, v := range data {
		result[k] = v.([]*discovery.Instance)
	}
	return result
}

func (d *kitcDiscoverer) fetchErrHandlerfunc(key string, err error) {
	logs.Errorf("KITC: service discover key: %s, err: %s", key, err.Error())
}

func (d *kitcDiscoverer) isSameInstances(key string, oldData, newData interface{}) bool {
	oldIns := oldData.([]*discovery.Instance)
	newIns := newData.([]*discovery.Instance)
	if len(oldIns) != len(newIns) {
		return false
	}
	if len(oldIns) == 0 && len(newIns) == 0 {
		return true
	}
	// compare instances which sorted by function fetch
	for i := range oldIns {
		// check address
		if oldIns[i].Address() != newIns[i].Address() {
			return false
		}
		// check tags
		if !d.isSameInstanceTags(oldIns[i], newIns[i]) {
			return false
		}
	}
	return true
}

func (d *kitcDiscoverer) isSameInstanceTags(oldData, newData *discovery.Instance) bool {
	if len(oldData.Tags) != len(newData.Tags) {
		return false
	}
	for k, oldVal := range oldData.Tags {
		if newVal, ok := newData.Tags[k]; !ok || oldVal != newVal {
			return false
		}
	}
	return true
}

// diffInstances merge compare oldIns with newIns, must be ordered
func (d *kitcDiscoverer) diffInstances(oldIns, newIns []*discovery.Instance) (addIns, modifyIns, deletedIns []*discovery.Instance) {
	var i, j int
	for i < len(oldIns) && j < len(newIns) {
		if oldIns[i].Address() == newIns[j].Address() {
			// check tags
			if !d.isSameInstanceTags(oldIns[i], newIns[j]) {
				modifyIns = append(modifyIns, newIns[j])
			}
			i++
			j++
			continue
		}
		if oldIns[i].Address() < newIns[j].Address() {
			deletedIns = append(deletedIns, oldIns[i])
			i++
		} else {
			addIns = append(addIns, newIns[j])
			j++
		}
	}
	if i < len(oldIns) {
		deletedIns = append(deletedIns, oldIns[i:]...)
	}
	if j < len(newIns) {
		addIns = append(addIns, newIns[j:]...)
	}
	return
}

func (d *kitcDiscoverer) instancesChangeHandler(key string, oldData, newData interface{}) {
	// oldIns and newIns is ordered, which sort by function fetch
	oldIns := oldData.([]*discovery.Instance)
	newIns := newData.([]*discovery.Instance)
	addIns, modifyIns, deletedIns := d.diffInstances(oldIns, newIns)

	d.eventBus.Dispatch(&kitevent.KitEvent{
		Name:   SERVICE_ADDRESS_CHANGE,
		Time:   time.Now(),
		Detail: fmt.Sprintf("%s: %s -> %s", key, instances2ReadableStr(oldIns), instances2ReadableStr(newIns)),
		Extra: map[string]interface{}{
			"key":     key,
			"new":     newIns,
			"old":     oldIns,
			"deleted": deletedIns,
		},
	})
	// used by debug info
	d.eventBus.Dispatch(&kitevent.KitEvent{
		Name:   DOWNSTREAM_INS_CHANGE,
		Time:   time.Now(),
		Detail: key,
		Extra: map[string]interface{}{
			"add":     instances2ReadableDetail(addIns),
			"modify":  instances2ReadableDetail(modifyIns),
			"deleted": instances2ReadableDetail(deletedIns),
		},
	})
}

// sortInstance is used for soring discovery.Instance
type sortInstance []*discovery.Instance

func (s sortInstance) Len() int {
	return len(s)
}

func (s sortInstance) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s sortInstance) Less(i, j int) bool {
	return s[i].Address() < s[j].Address()
}

func (d *kitcDiscoverer) fetch(key string) (interface{}, error) {
	tmp := strings.SplitN(key, ":", 4)
	if len(tmp) != 4 {
		return nil, fmt.Errorf("KITC: invalid key when discover: %s", key)
	}
	service, idc, cluster, env := tmp[0], tmp[1], tmp[2], tmp[3]

	ins, err := d.discover.Discover(service, idc)
	if err != nil {
		logs.Warnf("KITC: discover call failed, err:%v, service: %s, idc: %s", err, service, idc)
		return nil, err
	}
	if len(ins) == 0 {
		return nil, fmt.Errorf("KITC: no instance on discover, service:%s", key)
	}

	filtered := d.policy.Filter(ins, cluster, env)
	if len(filtered) == 0 {
		return nil, fmt.Errorf("KITC: no instance remains for %s, ins list: %s", key, instances2ReadableDetail(ins))
	}
	sort.Sort(sortInstance(filtered))
	return filtered, nil
}

func instances2ReadableDetail(ins []*discovery.Instance) string {
	if len(ins) == 0 {
		return "[]"
	}
	insStr := make([]string, 0, len(ins))
	for _, i := range ins {
		insStr = append(insStr, net.JoinHostPort(i.Host, i.Port)+fmt.Sprintf("{%v}", i.Tags))
	}
	return strings.Join(insStr, ",")
}

func instances2ReadableStr(ins []*discovery.Instance) string {
	if len(ins) == 0 {
		return "[]"
	}
	insStr := make([]string, 0, len(ins))
	for _, i := range ins {
		insStr = append(insStr, net.JoinHostPort(i.Host, i.Port))
	}
	return strings.Join(insStr, ",")
}

func newKitcDiscovererFromCtx(ctx context.Context) *kitcDiscoverer {
	var discoverer *kitcDiscoverer
	var underlying discovery.ServiceDiscoverer

	ebus := ctx.Value(KITC_EVENT_BUS_KEY).(kitevent.EventBus)
	insp := ctx.Value(KITC_INSPECTOR).(kitutil.Inspector)
	opts := ctx.Value(KITC_OPTIONS_KEY).(*Options)
	if len(opts.Instances) > 0 {
		underlying = discovery.NewCustomDiscoverer(opts.Instances)
	} else if opts.Discoverer != nil {
		underlying = opts.Discoverer
	} else if opts.IKService != nil {
		underlying = &ikserviceWrapper{opts.IKService}
	} else {
		underlying = discovery.NewConsulDiscoverer()
	}

	discoverer = newKitcDiscoverer(ebus, insp, underlying)
	discoverer.policy = discovery.NewDiscoveryPolicy()

	if opts.ClusterPolicy != nil {
		discoverer.policy.ClusterPolicy = opts.ClusterPolicy
	}
	if opts.EnvPolicy != nil {
		discoverer.policy.EnvPolicy = opts.EnvPolicy
	}
	return discoverer
}

// NewServiceDiscoverMW .
func NewServiceDiscoverMW(mwCtx context.Context) endpoint.Middleware {
	var discoverer = newKitcDiscovererFromCtx(mwCtx)
	return func(next endpoint.EndPoint) endpoint.EndPoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			rpcInfo := GetRPCInfo(ctx)
			if inss := rpcInfo.Instances(); len(inss) > 0 {
				return next(ctx, request)
			}

			ins, err := discoverer.Discover(rpcInfo.To, rpcInfo.TargetIDC(), rpcInfo.ToCluster, rpcInfo.Env)
			if err == nil && len(ins) == 0 {
				err = fmt.Errorf("discovery no result")
			}
			if err != nil {
				kerr := kiterrno.NewKiteError(
					kiterrno.KITE,
					kiterrno.ServiceDiscoverCode,
					fmt.Errorf("idc=%s service=%s cluster=%s env=%s err: %s",
						rpcInfo.TargetIDC(), rpcInfo.To, rpcInfo.ToCluster, rpcInfo.Env, err.Error()))
				return nil, kerr
			}

			rpcInfo.SetInstances(ins)
			return next(ctx, request)
		}
	}
}
