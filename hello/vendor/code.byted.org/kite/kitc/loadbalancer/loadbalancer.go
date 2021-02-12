package loadbalancer

import (
	"context"

	"code.byted.org/kite/kitc/discovery"
)

// Picker .
type Picker interface {
	Pick() (*discovery.Instance, bool)
}

// Loadbalancer Pick instance from iInstances, if there is no more instance,
// 	return false
type Loadbalancer interface {
	Name() string
	NewPicker(ctx context.Context, req interface{}, key string, instances []*discovery.Instance) Picker
	FallbackPicker(ctx context.Context, req interface{}, instances []*discovery.Instance) Picker
}

type Rebalancer interface {
	Rebalance(key string, newIns []*discovery.Instance)
	IsExist(key string) bool
}
