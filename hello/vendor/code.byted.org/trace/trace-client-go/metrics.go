// adapter between code.byted.org/gopkg/metrics and jaeger-lib/metrics

package trace

import (
	"time"

	"code.byted.org/gopkg/logs"
	bm "code.byted.org/gopkg/metrics"
	jm "code.byted.org/trace/trace-client-go/jaeger-lib/metrics"
)

const (
	TRACE_CLIENT_NAMESPACE = "toutiao.infra.trace.client"
)

type bytedMetricsFactory struct {
	ns   string
	mc   *bm.MetricsClientV2
	tags []bm.T
}

func NewBytedMetricsFactory(psm string, globalTags map[string]string) jm.Factory {

	ns := TRACE_CLIENT_NAMESPACE
	nmc := bm.NewDefaultMetricsClientV2(ns, true)

	ctags := make([]bm.T, 0, len(globalTags))
	for k, v := range globalTags {
		ctags = append(ctags, bm.Tag(k, v))
	}

	ctags = append(ctags, bm.Tag("language", "go"))

	return &bytedMetricsFactory{ns: ns, mc: nmc, tags: ctags}
}

func (mf *bytedMetricsFactory) Counter(name string, tags map[string]string) jm.Counter {
	return mf.newMetrics(name, tags)
}

func (mf *bytedMetricsFactory) Timer(name string, tags map[string]string) jm.Timer {
	return mf.newMetrics(name, tags)
}

func (mf *bytedMetricsFactory) Gauge(name string, tags map[string]string) jm.Gauge {
	return mf.newMetrics(name, tags)
}

func (mf *bytedMetricsFactory) Namespace(name string, tags map[string]string) jm.Factory {
	new_ns := mf.ns
	if name != "" {
		new_ns = new_ns + "." + name
	}
	nmc := bm.NewDefaultMetricsClientV2(new_ns, true)
	ctags := make([]bm.T, 0, len(tags)+len(mf.tags))
	for k, v := range tags {
		ctags = append(ctags, bm.Tag(k, v))
	}

	ctags = append(ctags, mf.tags...)
	return &bytedMetricsFactory{ns: new_ns, mc: nmc, tags: ctags}
}

func (mf *bytedMetricsFactory) newMetrics(name string, tags map[string]string) *BytedMetrics {
	ctags := make([]bm.T, 0, len(mf.tags)+len(tags))
	// assign namespace global tags
	ctags = append(ctags, mf.tags...)

	// assign parameter tags
	for k, v := range tags {
		ctags = append(ctags, bm.Tag(k, v))
	}

	return &BytedMetrics{
		mname: name,
		tags:  ctags,
		mc:    mf.mc,
	}
}

type BytedMetrics struct {
	mname string
	tags  []bm.T
	mc    *bm.MetricsClientV2
}

func (m *BytedMetrics) Inc(v int64) {
	if m.mc != nil {
		if err := m.mc.EmitCounter(m.mname, v, m.tags...); err != nil {
			logs.Warnf("EmitCounter for %s failed. ErrMsg: %s", m.mname, err.Error())
		}
	}
}

func (m *BytedMetrics) Record(d time.Duration) {
	if m.mc != nil {
		if err := m.mc.EmitTimer(m.mname, d, m.tags...); err != nil {
			logs.Warnf("EmitTimer for %s failed. ErrMsg: %s", m.mname, err.Error())
		}
	}
}

func (m *BytedMetrics) Update(v int64) {
	if m.mc != nil {
		if err := m.mc.EmitStore(m.mname, v, m.tags...); err != nil {
			logs.Warnf("EmitStore for %s failed. ErrMsg: %s", m.mname, err.Error())
		}
	}
}
