package loadbalancer

import (
	"context"

	"code.byted.org/kite/kitc/discovery"
)

type simplePicker struct {
	ins []*discovery.Instance
}

func (sp *simplePicker) Pick() (*discovery.Instance, bool) {
	if len(sp.ins) == 0 {
		return nil, false
	}

	ins := sp.ins[0]
	sp.ins = sp.ins[1:]
	return ins, true
}

// SimpleLoadbalancer .
type SimpleLoadbalancer struct{}

// Name .
func (sl *SimpleLoadbalancer) Name() string {
	return "SimpleLoadbalancer"
}

// NewPicker .
func (sl *SimpleLoadbalancer) NewPicker(ctx context.Context, req interface{}, instances []*discovery.Instance) Picker {
	return &simplePicker{instances}
}
