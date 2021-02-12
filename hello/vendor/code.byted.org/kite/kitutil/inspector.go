package kitutil

type Probe func(data map[string]interface{})

type Inspector interface {
	Register(probe Probe)
	Collect() map[string]interface{}
}

func NewInspector() Inspector {
	return &inspec{}
}

type inspec struct {
	probes []Probe
}

func (i *inspec) Register(probe Probe) {
	i.probes = append(i.probes, probe)
}

func (i *inspec) Collect() map[string]interface{} {
	data := make(map[string]interface{})
	for _, prb := range i.probes {
		prb(data)
	}
	return data
}
