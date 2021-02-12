package stats

import (
	"errors"
	"runtime"
	"time"

	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/metrics"
)

// deprecated: for kite & ms only ...
// for standalone service use NewStats
type StatsEmitor struct {
	mcli   *metrics.MetricsClientV2
	cgroup *CGroupCollector

	storeMetrics map[string]string
	timerMetrics map[string]string

	cgroupDirs map[string]string

	tags []metrics.T
}

func NewStatsEmitor(cli *metrics.MetricsClientV2) (*StatsEmitor, error) {
	r := &StatsEmitor{}
	r.mcli = cli
	if err := r.init(); err != nil {
		return nil, err
	}
	r.cgroup, _ = NewCGroupColletor()
	r.tags = []metrics.T{{
		Name:  "cluster",
		Value: env.Cluster(),
	}}
	return r, nil
}

// deprecated: for kite & ms only ...
func DoReport(name string) error {
	mcli := metrics.NewDefaultMetricsClientV2("go."+name, true)
	se, err := NewStatsEmitor(mcli)
	if err != nil {
		return err
	}
	go se.Loop()
	return nil
}

func (r *StatsEmitor) init() error {
	r.storeMetrics = map[string]string{
		"heap.byte":           "HeapAlloc",
		"stack.byte":          "StackInuse",
		"numGcs":              "NumGC",
		"numGos":              "Goroutines",
		"malloc":              "Mallocs",
		"free":                "Frees",
		"totalAllocated.byte": "TotalAlloc",
		"objects":             "HeapObjects",
	}
	r.timerMetrics = map[string]string{
		"gcPause.us": "GCPauseUs",
		"gcCPU":      "GCCPUFraction",
	}
	var s StatItem
	for _, f := range r.storeMetrics {
		if s.GetField(f) == nil {
			return errors.New("[BUG] field " + f + " not found")
		}
	}
	for _, f := range r.timerMetrics {
		if s.GetField(f) == nil {
			return errors.New("[BUG] field " + f + " not found")
		}
	}
	return nil
}

type StatItem struct {
	runtime.MemStats
	Goroutines int64
	NumGC      int64
	GCPauseUs  uint64
}

// GetField is util for emit metrics
func (e StatItem) GetField(key string) interface{} {
	switch key {
	case "HeapAlloc":
		return e.HeapAlloc
	case "StackInuse":
		return e.StackInuse
	case "NumGC":
		return e.NumGC
	case "Goroutines":
		return e.Goroutines
	case "TotalAlloc":
		return e.TotalAlloc
	case "Mallocs":
		return e.Mallocs
	case "Frees":
		return e.Frees
	case "HeapObjects":
		return e.HeapObjects
	case "GCCPUFraction":
		return e.GCCPUFraction
	case "GCPauseUs":
		return e.GCPauseUs
	}
	return nil
}

func (r *StatsEmitor) TickStat(d time.Duration) <-chan StatItem {
	var m0, m1 runtime.MemStats
	ret := make(chan StatItem)
	go func() {
		runtime.ReadMemStats(&m0)
		for {
			<-time.After(d)
			runtime.ReadMemStats(&m1)
			ret <- StatItem{
				MemStats:   m1,
				Goroutines: int64(runtime.NumGoroutine()),
				NumGC:      int64(m1.NumGC - m0.NumGC),
				GCPauseUs:  GCPauseNs(m1, m0) / 1000,
			}
			m0 = m1
		}
	}()
	return ret
}

func (r *StatsEmitor) Loop() {
	var st0 CGroupCPUStat
	if r.cgroup != nil {
		r.cgroup.ReadCPUStat(&st0)
	}
	for e := range r.TickStat(10 * time.Second) {
		for k, f := range r.storeMetrics {
			r.mcli.EmitStore(k, e.GetField(f), r.tags...)
		}
		for k, f := range r.timerMetrics {
			r.mcli.EmitTimer(k, e.GetField(f), r.tags...)
		}
		if r.cgroup == nil {
			continue
		}
		var st CGroupCPUStat
		r.cgroup.ReadCPUStat(&st)
		if n := st.NrThrottled - st0.NrThrottled; n > 0 {
			r.mcli.EmitCounter("cgroup.cpustat.nr_throttled", n, r.tags...)
		}
		if n := st.NrPeriods - st0.NrPeriods; n > 0 {
			r.mcli.EmitCounter("cgroup.cpustat.nr_periods", n, r.tags...)
		}
		st0 = st
	}
}
