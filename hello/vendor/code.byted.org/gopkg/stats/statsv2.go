package stats

import (
	"runtime"
	"time"

	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/metrics"
)

type Stats struct {
	mcli   *metrics.MetricsClientV2
	cgroup *CGroupCollector
	tags   []metrics.T
}

func NewStats(mcli *metrics.MetricsClientV2) *Stats {
	s := new(Stats)
	s.mcli = mcli
	s.cgroup, _ = NewCGroupColletor()
	s.tags = []metrics.T{{
		Name:  "cluster",
		Value: env.Cluster(),
	}}
	return s
}

type EmitType uint32

const (
	// counter: cgroup.cpustat.nr_throttled
	// counter: cgroup.cpustat.nr_periods
	// counter: cgroup.cputime
	EmitCGroupCPUStat EmitType = 1 << iota

	// counter: runtime.alloc.objects
	EmitGoAllocObjectsCounter

	// counter: runtime.alloc.bytes
	EmitGoAllocBytesCounter

	// timer: runtime.gc.pause
	EmitGoGCPause

	// counter: runtime.gc.count
	EmitGoGCNum

	// counter: runtime.cgocall.count
	EmitCgoCall

	// store: runtime.goroutine.num
	EmitGoroutineNum

	EmitDefault = EmitGoAllocObjectsCounter | EmitGoAllocBytesCounter | EmitGoGCPause | EmitGoGCNum | EmitCgoCall | EmitGoroutineNum
	EmitCGroup  = EmitCGroupCPUStat
	EmitAll     = EmitDefault | EmitCGroup
)

func (s *Stats) EmitLoop(d time.Duration, ets ...EmitType) {
	var et EmitType
	for _, t := range ets {
		et |= t
	}
	if et == 0 {
		et = EmitDefault
	}

	var m0, m1 runtime.MemStats
	var s0, s1 CGroupCPUStat

	var lastCgoCalls int64

	runtime.ReadMemStats(&m0)
	if et&EmitCGroupCPUStat != 0 {
		s.cgroup.ReadCPUStat(&s0)
	}
	for range time.Tick(d) {
		runtime.ReadMemStats(&m1)
		if et&EmitGoAllocObjectsCounter != 0 && m1.Mallocs-m0.Mallocs > 0 {
			s.mcli.EmitCounter("runtime.alloc.objects", m1.Mallocs-m0.Mallocs, s.tags...)
		}
		if et&EmitGoAllocBytesCounter != 0 && m1.TotalAlloc-m0.TotalAlloc > 0 {
			s.mcli.EmitCounter("runtime.alloc.bytes", m1.TotalAlloc-m0.TotalAlloc, s.tags...)
		}
		if et&EmitGoGCPause != 0 {
			s.mcli.EmitTimer("runtime.gc.pause", GCPauseNs(m1, m0), s.tags...)
		}
		if et&EmitGoGCNum != 0 && m1.NumGC-m0.NumGC > 0 {
			s.mcli.EmitCounter("runtime.gc.count", m1.NumGC-m0.NumGC, s.tags...)
			s.mcli.EmitCounter("runtime.gc.num", m1.NumGC-m0.NumGC, s.tags...) // FIXME: to be deleted in future
		}
		if et&EmitGoroutineNum != 0 {
			s.mcli.EmitStore("runtime.goroutine.num", runtime.NumGoroutine(), s.tags...)
		}
		if et&EmitCgoCall != 0 {
			calls := runtime.NumCgoCall()
			if n := calls - lastCgoCalls; n > 0 {
				s.mcli.EmitCounter("runtime.cgocall.count", n, s.tags...)
			}
			lastCgoCalls = calls
		}
		if et&EmitCGroupCPUStat != 0 {
			s.cgroup.ReadCPUStat(&s1)
			if n := s1.NrThrottled - s0.NrThrottled; n > 0 {
				s.mcli.EmitCounter("cgroup.cpustat.nr_throttled", n, s.tags...)
			}
			if n := s1.NrPeriods - s0.NrPeriods; n > 0 {
				s.mcli.EmitCounter("cgroup.cpustat.nr_periods", n, s.tags...)
			}
			if n := s1.CPUTime - s0.CPUTime; n > 0 {
				s.mcli.EmitCounter("cgroup.cputime", n, s.tags...)
			}
		}
		m0 = m1
		s0 = s1
	}
}
