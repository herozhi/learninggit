package loadbalancer

import (
	"context"
	"sync"

	"code.byted.org/gopkg/rand"
	"code.byted.org/kite/kitc/discovery"
)

type randomPicker struct {
	immutableInstances []*discovery.Instance
	firstIndex         int
	copiedInstances    []*discovery.Instance
}

func (bp *randomPicker) Pick() (*discovery.Instance, bool) {
	if bp.firstIndex < 0 {
		bp.firstIndex = rand.Intn(len(bp.immutableInstances))
		return bp.immutableInstances[bp.firstIndex], true
	}

	if bp.copiedInstances == nil {
		bp.copiedInstances = make([]*discovery.Instance, len(bp.immutableInstances)-1)
		copy(bp.copiedInstances, bp.immutableInstances[:bp.firstIndex])
		copy(bp.copiedInstances[bp.firstIndex:], bp.immutableInstances[bp.firstIndex+1:])
	}

	n := len(bp.copiedInstances)
	if n > 0 {
		index := rand.Intn(n)
		ins := bp.copiedInstances[index]
		bp.copiedInstances[index] = bp.copiedInstances[n-1]
		bp.copiedInstances = bp.copiedInstances[:n-1]
		return ins, true
	}

	return nil, false
}

type weightPicker struct {
	immutableInstances []*discovery.Instance
	immutableWeights   []int
	weightSum          int

	firstIndex      int
	copiedInstances []*discovery.Instance
	copiedWeights   []int
}

func (wb *weightPicker) Pick() (*discovery.Instance, bool) {
	if wb.weightSum == 0 {
		return nil, false
	}

	if wb.firstIndex < 0 {
		weight := rand.Intn(wb.weightSum)
		for i := 0; i < len(wb.immutableWeights); i++ {
			weight -= wb.immutableWeights[i]
			if weight < 0 {
				wb.firstIndex = i
				break
			}
		}
		return wb.immutableInstances[wb.firstIndex], true
	}

	if wb.copiedInstances == nil {
		wb.copiedInstances = make([]*discovery.Instance, len(wb.immutableInstances)-1)
		copy(wb.copiedInstances, wb.immutableInstances[:wb.firstIndex])
		copy(wb.copiedInstances[wb.firstIndex:], wb.immutableInstances[wb.firstIndex+1:])

		wb.copiedWeights = make([]int, len(wb.immutableWeights)-1)
		copy(wb.copiedWeights, wb.immutableWeights[:wb.firstIndex])
		copy(wb.copiedWeights[wb.firstIndex:], wb.immutableWeights[wb.firstIndex+1:])

		wb.weightSum -= wb.immutableWeights[wb.firstIndex]
	}

	n := len(wb.copiedInstances)
	if n > 0 {
		weight := rand.Intn(wb.weightSum)
		for i := 0; i < len(wb.copiedWeights); i++ {
			weight -= wb.copiedWeights[i]
			if weight < 0 {
				wb.weightSum -= wb.copiedWeights[i]
				ins := wb.copiedInstances[i]
				wb.copiedInstances[i] = wb.copiedInstances[n-1]
				wb.copiedInstances = wb.copiedInstances[:n-1]
				wb.copiedWeights[i] = wb.copiedWeights[n-1]
				wb.copiedWeights = wb.copiedWeights[:n-1]
				return ins, true
			}
		}
	}
	return nil, false
}

type weightInstanceInfo struct {
	instances  []*discovery.Instance
	weightList []int
	weightSum  int
	balance    bool
}

// WeightLoadbalancer .
type WeightLoadbalancer struct {
	instancesInfo map[string]*weightInstanceInfo
	lock          sync.RWMutex
}

func NewWeightLoadbalancer() *WeightLoadbalancer {
	return &WeightLoadbalancer{
		instancesInfo: make(map[string]*weightInstanceInfo),
	}
}

// Name .
func (wl *WeightLoadbalancer) Name() string {
	return "WeightLoadbalancer"
}

// For fallback
func (wl *WeightLoadbalancer) FallbackPicker(ctx context.Context, req interface{},
	ins []*discovery.Instance) Picker {
	weightList := make([]int, len(ins))
	sum := 0
	for i, in := range ins {
		weightList[i] = in.Weight()
		sum += weightList[i]
	}
	return &weightPicker{
		immutableInstances: ins,
		immutableWeights:   weightList,
		weightSum:          sum,
		firstIndex:         -1,
	}
}

// NewPicker .
func (wl *WeightLoadbalancer) NewPicker(ctx context.Context, req interface{},
	key string, instances []*discovery.Instance) Picker {

	if !wl.IsExist(key) {
		return wl.FallbackPicker(ctx, req, instances)
	}

	wl.lock.RLock()
	defer wl.lock.RUnlock()

	if wl.instancesInfo[key].balance {
		return &randomPicker{
			immutableInstances: wl.instancesInfo[key].instances,
			firstIndex:         -1,
		}
	}

	return &weightPicker{
		immutableInstances: wl.instancesInfo[key].instances,
		immutableWeights:   wl.instancesInfo[key].weightList,
		weightSum:          wl.instancesInfo[key].weightSum,
		firstIndex:         -1,
	}
}

func (wl *WeightLoadbalancer) Rebalance(key string, newIns []*discovery.Instance) {
	if len(newIns) <= 0 {
		return
	}

	weightList := make([]int, len(newIns))
	weightSum := 0
	maxWeight := -1
	minWeight := 1 << 31
	for i, in := range newIns {
		weightList[i] = in.Weight()
		weightSum += weightList[i]

		if weightList[i] > maxWeight {
			maxWeight = weightList[i]
		}
		if weightList[i] < minWeight {
			minWeight = weightList[i]
		}
	}
	balance := maxWeight == minWeight

	wl.lock.Lock()
	insInfo := new(weightInstanceInfo)
	insInfo.instances = newIns
	insInfo.balance = balance
	insInfo.weightList = weightList
	insInfo.weightSum = weightSum
	wl.instancesInfo[key] = insInfo
	wl.lock.Unlock()
}

func (wl *WeightLoadbalancer) IsExist(key string) bool {
	wl.lock.RLock()
	_, ok := wl.instancesInfo[key]
	wl.lock.RUnlock()
	return ok
}
