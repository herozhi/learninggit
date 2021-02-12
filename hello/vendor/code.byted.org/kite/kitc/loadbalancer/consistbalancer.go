package loadbalancer

import (
	"context"
	"fmt"
	"net"
	"sort"
	"sync"

	"code.byted.org/kite/kitc/discovery"
	"github.com/cespare/xxhash"
)

type KeyFunc func(ctx context.Context, req interface{}) (string, error)

/*
	ConsistBalancer
	只能尽量保持一致性;
	同一个key在某些情况下, 也会打倒不同的机器, 主要原因有:
	1. 下游实例地址发送变化, 为了负载均匀, 需要rebalance, 于是某些key的对应实例会变;
	2. 访问下游可能失败, 出现链接失败时, 会重试其他机器, 导致同一个key打到不同的机器;
*/
type ConsistBalancer struct {
	GetKey   KeyFunc
	backup   Loadbalancer
	replicas int

	instanceInfo map[string][]*discovery.Instance
	sortedHash   []uint64
	hash2Ins     map[uint64]*discovery.Instance
	lock         sync.RWMutex
}

func NewConsistBalancer(GetKey KeyFunc) *ConsistBalancer {
	return &ConsistBalancer{
		GetKey:       GetKey,
		backup:       NewWeightLoadbalancer(),
		instanceInfo: make(map[string][]*discovery.Instance),
		hash2Ins:     make(map[uint64]*discovery.Instance),
		replicas:     23,
	}
}

func (cb *ConsistBalancer) Name() string {
	return "ConsistBalancer"
}

// should never call ConsistBalancer FallbackPicker
func (cb *ConsistBalancer) FallbackPicker(ctx context.Context, req interface{},
	instances []*discovery.Instance) Picker {
	return &consistBalancerPicker{nil}
}

func (cb *ConsistBalancer) NewPicker(ctx context.Context, req interface{}, key string,
	instances []*discovery.Instance) Picker {

	if !cb.IsExist(key) {
		return cb.backup.FallbackPicker(ctx, req, instances)
	}

	hashKey, err := cb.GetKey(ctx, req)
	if err != nil {
		return cb.backup.FallbackPicker(ctx, req, instances)
	}

	insList := cb.search(hash(hashKey))
	if len(insList) == 0 {
		return cb.backup.FallbackPicker(ctx, req, instances)
	}

	return &consistBalancerPicker{insList}
}

func (cb *ConsistBalancer) search(hashCode uint64) []*discovery.Instance {
	results := make([]*discovery.Instance, 0, 3)
	dup := make(map[string]struct{}, 3) // 消除replication的影响
	cb.lock.RLock()
	defer cb.lock.RUnlock()

	index := sort.Search(len(cb.sortedHash), func(x int) bool {
		return cb.sortedHash[x] > hashCode
	})
	if index == len(cb.sortedHash) {
		index = 0
	}

	for i := 0; i < len(cb.sortedHash); i++ {
		ins := cb.hash2Ins[cb.sortedHash[index]]
		key := instanceKey(ins)
		if _, ok := dup[key]; ok {
			index = (index + 1) % len(cb.sortedHash)
			continue
		}

		dup[key] = struct{}{}
		results = append(results, ins)
		index = (index + 1) % len(cb.sortedHash)

		if len(results) == 3 {
			break
		}
	}

	return results
}

func (cb *ConsistBalancer) IsExist(key string) bool {
	cb.lock.RLock()
	_, ok := cb.instanceInfo[key]
	cb.lock.RUnlock()
	return ok
}

func (cb *ConsistBalancer) Rebalance(key string, insList []*discovery.Instance) {
	var oldInsList []*discovery.Instance

	if !cb.IsExist(key) {
		goto DoRebalance
	}

	cb.lock.RLock()
	oldInsList = cb.instanceInfo[key]
	cb.lock.RUnlock()

	if len(insList) == len(oldInsList) {
		newMap, oldMap := make(map[string]struct{}, len(insList)), make(map[string]struct{}, len(oldInsList))
		for _, ins := range insList {
			newMap[instanceKey(ins)] = struct{}{}
		}
		for _, ins := range oldInsList {
			oldMap[instanceKey(ins)] = struct{}{}
		}
		if len(newMap) == len(oldMap) {
			for k := range newMap {
				if _, ok := oldMap[k]; !ok {
					goto DoRebalance
				}
			}
			return // same instance list
		}
	}

DoRebalance:
	cb.rebalance(key, insList)
}

func (cb *ConsistBalancer) rebalance(key string, insList []*discovery.Instance) {
	cb.lock.Lock()
	cb.instanceInfo[key] = insList

	totalLen := 0
	for _, instances := range cb.instanceInfo {
		totalLen += len(instances)
	}
	sortedHash := make([]uint64, 0, totalLen*cb.replicas)
	hash2Ins := make(map[uint64]*discovery.Instance, totalLen*cb.replicas)

	for _, insList := range cb.instanceInfo {
		for _, ins := range insList {
			for i := 0; i < cb.replicas; i++ {
				key := fmt.Sprintf("%v:%v", instanceKey(ins), i)
				hashCode := hash(key)
				sortedHash = append(sortedHash, hashCode)
				hash2Ins[hashCode] = ins
			}
		}
	}
	sort.Slice(sortedHash, func(i, j int) bool {
		return sortedHash[i] < sortedHash[j]
	})

	cb.hash2Ins = hash2Ins
	cb.sortedHash = sortedHash
	cb.lock.Unlock()
}

type consistBalancerPicker struct {
	insList []*discovery.Instance
}

func (cbp *consistBalancerPicker) Pick() (*discovery.Instance, bool) {
	if len(cbp.insList) == 0 {
		return nil, false
	}
	ret := cbp.insList[0]
	cbp.insList = cbp.insList[1:]
	return ret, true
}

func instanceKey(ins *discovery.Instance) string {
	return net.JoinHostPort(ins.Host, ins.Port)
}

func hash(key string) uint64 {
	return xxhash.Sum64String(key)
}
