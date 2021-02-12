package discovery

type customDiscoverer struct {
	insList []*Instance
}

// if tags == nil, that equal all idcs.
func (s *customDiscoverer) Discover(servideName, idc string) ([]*Instance, error) {
	var ret []*Instance
	for _, ins := range s.insList {
		if _, ok := ins.Tags["idc"]; !ok || ins.Tags == nil || ins.Tags["idc"] == idc {
			ret = append(ret, ins)
		}
	}
	return ret, nil
}

// NewCustomDiscoverer .
func NewCustomDiscoverer(ins []*Instance) ServiceDiscoverer {
	return &customDiscoverer{
		insList: ins,
	}
}
