package trace

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"crypto/md5"
	"math/big"

	"math"
	"reflect"

	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/net2"
	kext "code.byted.org/trace/trace-client-go/ext"
	j "code.byted.org/trace/trace-client-go/jaeger-client"
	jm "code.byted.org/trace/trace-client-go/jaeger-lib/metrics"
	"code.byted.org/trace/trace-client-go/utils"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

var (
	globalMetricsFactory              = jm.NullFactory
	localHostIP                uint64 = 0
	defaultAppRegistryCapacity int    = 10
	defaultDebugFromTLBEnable  int32  = 1
)

var NoopSpanTypeName string

func init() {
	if hostip := env.HostIP(); "" != hostip && hostip != net2.UnknownIPAddr {
		localHostIP = uint64(utils.InetAtoN(hostip))
	}
	noopSpan := opentracing.NoopTracer{}.StartSpan("noop")
	NoopSpanTypeName = reflect.TypeOf(noopSpan).Name()
}

func Init(serviceName string) error {
	metricsFactory := NewBytedMetricsFactory(serviceName, nil)
	metrics := j.NewMetrics(metricsFactory, nil)
	sampler := NewEtcdSampler(serviceName,
		SamplerOptions.Metrics(metrics),
		SamplerOptions.Logger(BytedLogger))
	sender, err := NewGosdkTransport(defaultTaskName)

	if err == nil {
		reporter := j.NewRemoteReporter(sender,
			j.ReporterOptions.Metrics(metrics),
			j.ReporterOptions.Logger(BytedLogger),
		)
		tracer, _ := j.NewTracer(
			serviceName, sampler, reporter,
			j.TracerOptions.Metrics(metrics),
			j.TracerOptions.Logger(BytedLogger),
			j.TracerOptions.PoolSpans(false),
			j.TracerOptions.HighTraceIDGenerator(func() uint64 { return localHostIP }),
			j.TracerOptions.PostTraceIDGenerator(
				func(options opentracing.StartSpanOptions) (traceID j.TraceID) {
					iLogID, ok := options.Tags[string(kext.RPCLogID)]
					if !ok {
						return
					}
					logID, ok := iLogID.(string)
					if !ok {
						return
					}

					if len(logID) <= 0 {
						return
					}
					return hashToTraceID([]byte(logID))
				},
			),
			j.TracerOptions.Gen128Bit(true),
			j.TracerOptions.DebugThrottler(newBytedThrottler(metrics)),
		)
		btracer := newBytedTracer(tracer, metrics)
		opentracing.SetGlobalTracer(btracer)
		sampler.start()
	}

	return err
}

func Close() error {
	if btracer, ok := opentracing.GlobalTracer().(*bytedTracer); ok {
		return btracer.Close()
	}
	return nil
}

func ForceTrace(span opentracing.Span) bool {
	ext.SamplingPriority.Set(span, uint16(0x1))

	if jspan, ok := span.(kext.SpanWithSampleFlag); ok {
		return jspan.IsSampled()
	}

	return false
}

func IsSampled(span opentracing.Span) bool {
	if jspan, ok := span.(kext.SpanWithSampleFlag); ok {
		return jspan.IsSampled()
	}

	return false
}

func IsDebug(span opentracing.Span) bool {
	if jspan, ok := span.(*j.Span); ok {
		return jspan.IsDebug()
	}

	return false
}

func IsJaegerSpan(span opentracing.Span) bool {
	_, ok := span.(*j.Span)
	return ok
}

func JSpanContextToString(spanCtx opentracing.SpanContext) string {
	if jctx, ok := spanCtx.(j.SpanContext); ok {
		return jctx.String()
	}

	return ""
}

func FormatOperationName(serviceName, operationName string) string {
	if serviceName == "" || serviceName == "-" {
		serviceName = "unknown"
	}
	if operationName == "" || operationName == "unknown_method" {
		operationName = "-"
	}

	return serviceName + "::" + operationName
}

func SetOperationName(span opentracing.Span, operation string) bool {
	if jspan, ok := span.(*j.Span); ok {
		jspan.SetOperationName(operation)
		return true
	}
	return false
}

func FillSpanEvent(ctx context.Context, event kext.EventKindEnum) error {
	span := opentracing.SpanFromContext(ctx)
	if span == nil {
		return fmt.Errorf("span is nil, fill event-%v into span failed", event)
	}
	if !IsSampled(span) {
		return nil
	}

	switch event {
	case kext.EventKindConnectStartEnum:
		span.LogFields(kext.EventKindConnectStart)
	case kext.EventKindConnectEndEnum:
		span.LogFields(kext.EventKindConnectEnd)
	case kext.EventKindPkgSendStartEnum:
		span.LogFields(kext.EventKindPkgSendStart)
	case kext.EventKindPkgSendEndEnum:
		span.LogFields(kext.EventKindPkgSendEnd)
	case kext.EventKindPkgRecvStartEnum:
		span.LogFields(kext.EventKindPkgRecvStart)
	case kext.EventKindPkgRecvEndEnum:
		span.LogFields(kext.EventKindPkgRecvEnd)
	default:
		return fmt.Errorf("not supported EventKind: %v", event)
	}

	return nil
}

func DyeUUID(uuid int64) bool {
	if btracer, ok := opentracing.GlobalTracer().(*bytedTracer); ok {
		btracer.RLock()
		defer btracer.RUnlock()
		return btracer.dyeUUIDSet[uuid] || btracer.centralizedDyeUUIDSet[uuid]
	}
	return false
}

func DyeAppUUID(appid int32, uuid int64) bool {
	if btracer, ok := opentracing.GlobalTracer().(*bytedTracer); ok {
		return btracer.dyeAppUUID(appid, uuid)
	}
	return false
}

func AllowTraceTagFromTLB() bool {
	if btracer, ok := opentracing.GlobalTracer().(*bytedTracer); ok {
		return btracer.allowTraceTagFromTLB()
	}
	return false
}

func RootSpanEnable(enable int32, from ConfigSourceType) bool {
	if btracer, ok := opentracing.GlobalTracer().(*bytedTracer); ok {
		return btracer.RootSpanEnable(enable, from)
	}
	return false
}

func UpdateDyeUUIDAppRegistryCapacity(cap int) bool {
	if btracer, ok := opentracing.GlobalTracer().(*bytedTracer); ok {
		return btracer.UpdateDyeUUIDAppRegistryCapacity(cap)
	}
	return false
}

func UpdateDyeUUIDSet(uuidSet map[int64]bool) bool {
	if btracer, ok := opentracing.GlobalTracer().(*bytedTracer); ok {
		return btracer.updateDyeUUIDSet(uuidSet)
	}
	return false
}

func UpdateBytedTracerFromRemoteConf(c *ServiceSamplingStrategy) bool {
	if btracer, ok := opentracing.GlobalTracer().(*bytedTracer); ok {
		return btracer.updateBytedTracerFromRemoteConf(c)
	}
	return false
}

func UpdateCentralizedDyeUUIDSet(uuidSet map[int64]bool) bool {
	if btracer, ok := opentracing.GlobalTracer().(*bytedTracer); ok {
		return btracer.updateCentralizedDyeUUIDSet(uuidSet)
	}
	return false
}

func UpdateDyeAppUUIDSet(dyeAppUUIDSet map[int32]map[int64]bool, appUUIDLastValue map[int32]string) bool {
	if btracer, ok := opentracing.GlobalTracer().(*bytedTracer); ok {
		return btracer.updateDyeAppUUIDSet(dyeAppUUIDSet, appUUIDLastValue)
	}
	return false
}

func PostTraceIsBlock(operationName, peerService string) bool {
	if btracer, ok := opentracing.GlobalTracer().(*bytedTracer); ok {

		if len(peerService) > 0 {
			if downstreamPostSpansLimiter.IsBlock(peerService) {
				btracer.metrics.DownstreamRateLimitedPostSpans.Inc(1)
				return true
			}
		}

		if globalPostSpansLimiter.IsBlock(operationName) {
			btracer.metrics.GlobalRateLimitedPostSpans.Inc(1)
			return true
		}
		return false
	}
	return true // only bytedTracer can post trace
}

func getAppRegistryAndDyeAppUUIDSet() (map[int32]string, map[int32]map[int64]bool) {
	if btracer, ok := opentracing.GlobalTracer().(*bytedTracer); ok {
		btracer.RLock()
		defer btracer.RUnlock()
		ret := make(map[int32]string, len(btracer.appRegistry))
		for k, v := range btracer.appRegistry {
			if v {
				ret[k] = btracer.appUUIDLastValue[k]
			}
		}

		for k, v := range btracer.appUUIDLastValue {
			if len(ret) >= btracer.appRegistryCapacity {
				break
			}
			ret[k] = v
		}

		return ret, btracer.dyeAppUUIDSet
	}
	return nil, nil
}

func hashToTraceID(data []byte) (id j.TraceID) {
	h := md5.New()
	h.Write(data)
	md5sum := h.Sum(nil)

	bi := big.NewInt(0)

	bi.SetBytes(md5sum[0:8])
	id.High = bi.Uint64() & math.MaxInt64 // 保证ID>=0且不超过MAX_INT64

	bi.SetBytes(md5sum[8:16])
	id.Low = bi.Uint64() & math.MaxInt64 // 保证ID>=0且不超过MAX_INT64

	return
}

// ---------------
// following is bytedTracer releated impl

type ConfigSourceType int32

const (
	CONFIG_FROM_FUNCTION ConfigSourceType = iota
	CONFIG_FROM_REMOTECENTER
	CONFIG_FROM_CONFFILE
	CONFIG_FROM_DEFAULT
)

type bytedTracer struct {
	sync.RWMutex
	opentracing.Tracer
	closed                int64
	rootSpanEnable        int32
	dyeUUIDSet            map[int64]bool
	centralizedDyeUUIDSet map[int64]bool
	rseSetBy              ConfigSourceType // root span enable config set by
	metrics               *j.Metrics

	appRegistry         map[int32]bool
	appRegistryCapacity int

	appUUIDLastValue map[int32]string
	dyeAppUUIDSet    map[int32]map[int64]bool

	debugFromTLBEnable int32
}

type bytedThrottler struct {
	metrics *j.Metrics
}

func (t *bytedThrottler) IsAllowed(operation string) bool {
	if globalLimiter.IsBlock(operation) {
		t.metrics.RateLimitedSpans.Inc(1)
		return false
	}
	return true
}

func newBytedThrottler(m *j.Metrics) *bytedThrottler {
	return &bytedThrottler{m}
}

func newBytedTracer(tracer opentracing.Tracer, metrics *j.Metrics) *bytedTracer {
	return &bytedTracer{
		closed:                0,
		Tracer:                tracer,
		rootSpanEnable:        0,
		dyeUUIDSet:            nil,
		centralizedDyeUUIDSet: nil,
		rseSetBy:              CONFIG_FROM_DEFAULT,
		metrics:               metrics,
		appRegistry:           make(map[int32]bool),
		appRegistryCapacity:   defaultAppRegistryCapacity,
		debugFromTLBEnable:    defaultDebugFromTLBEnable,
	}
}

func (t *bytedTracer) Close() error {
	if swapped := atomic.CompareAndSwapInt64(&t.closed, 0, 1); !swapped {
		return fmt.Errorf("repeated attempt to close the sender is ignored")
	}
	if jt, ok := t.Tracer.(*j.Tracer); ok {
		return jt.Close()
	}
	return nil
}

// configure priority: function > config file > remote config center > default
func (t *bytedTracer) RootSpanEnable(enable int32, from ConfigSourceType) bool {
	if from <= t.rseSetBy {
		t.Lock()
		defer t.Unlock()
		if from <= t.rseSetBy {
			// double check when holding lock
			atomic.StoreInt32(&t.rootSpanEnable, enable)
			t.rseSetBy = from
			return true
		}
	}
	return false
}

func (t *bytedTracer) UpdateDyeUUIDAppRegistryCapacity(cap int) bool {
	t.Lock()
	defer t.Unlock()
	t.appRegistryCapacity = cap
	return true
}

func (t *bytedTracer) updateDyeUUIDSet(uuidSet map[int64]bool) bool {
	t.Lock()
	defer t.Unlock()
	t.dyeUUIDSet = uuidSet
	return true
}

func (t *bytedTracer) updateBytedTracerFromRemoteConf(c *ServiceSamplingStrategy) bool {

	dyeUUIDSet := make(map[int64]bool)
	if len(c.InnerUUIDList) != 0 || len(c.DyeUUIDList) != 0 {
		for _, uuid := range c.InnerUUIDList {
			dyeUUIDSet[uuid] = true
		}
		for _, uuid := range c.DyeUUIDList {
			dyeUUIDSet[uuid] = true
		}
	}

	t.Lock()
	defer t.Unlock()

	// update root span enable & dye uuid list
	if c.RootSpanEnable != nil && CONFIG_FROM_REMOTECENTER <= t.rseSetBy {
		// double check when holding lock
		atomic.StoreInt32(&t.rootSpanEnable, *c.RootSpanEnable)
		t.rseSetBy = CONFIG_FROM_REMOTECENTER
	}

	// update DyeUUIDAppRegistryCapacity
	appRegistryCapacity := defaultAppRegistryCapacity
	if c.DyeUUIDAppRegistryCapacity != nil {
		appRegistryCapacity = *c.DyeUUIDAppRegistryCapacity
	}
	t.appRegistryCapacity = appRegistryCapacity

	t.dyeUUIDSet = dyeUUIDSet

	debugFromTLBEnable := defaultDebugFromTLBEnable
	if c.DebugFromTLBEnable != nil {
		debugFromTLBEnable = *c.DebugFromTLBEnable
	}

	atomic.StoreInt32(&t.debugFromTLBEnable, debugFromTLBEnable)
	return true
}

func (t *bytedTracer) updateCentralizedDyeUUIDSet(uuidSet map[int64]bool) bool {
	t.Lock()
	defer t.Unlock()
	t.centralizedDyeUUIDSet = uuidSet
	return true
}

func (t *bytedTracer) dyeAppUUID(appid int32, uuid int64) bool {
	t.RLock()

	if len(t.appRegistry) < t.appRegistryCapacity && !t.appRegistry[appid] {
		t.RUnlock()
		t.Lock()
		defer t.Unlock()
		if len(t.appRegistry) < t.appRegistryCapacity {
			t.appRegistry[appid] = true
		}
	} else {
		defer t.RUnlock()
	}

	return t.dyeAppUUIDSet[appid][uuid]
}

func (t *bytedTracer) allowTraceTagFromTLB() bool {
	debugFromTLBEnable := atomic.LoadInt32(&t.debugFromTLBEnable)
	return debugFromTLBEnable != 0
}

func (t *bytedTracer) updateDyeAppUUIDSet(dyeAppUUIDSet map[int32]map[int64]bool, appUUIDLastValue map[int32]string) bool {
	t.Lock()
	defer t.Unlock()
	t.appRegistry = make(map[int32]bool, defaultAppRegistryCapacity)
	t.dyeAppUUIDSet = dyeAppUUIDSet
	t.appUUIDLastValue = appUUIDLastValue
	return true
}

func (t *bytedTracer) StartSpan(operationName string,
	opts ...opentracing.StartSpanOption) opentracing.Span {

	opts = append(opts, kext.TraceClientVersionTag)

	rootSpanEnable := atomic.LoadInt32(&t.rootSpanEnable)
	var span opentracing.Span
	if rootSpanEnable != 0 {
		span = t.Tracer.StartSpan(operationName, opts...)
	} else {
		childof := false
		isPostTrace := false

	OptsLoop:
		for _, opt := range opts {
			switch opt.(type) {
			case opentracing.SpanReference:
				ref := opt.(opentracing.SpanReference)
				if ref.Type == opentracing.ChildOfRef || ref.Type == opentracing.FollowsFromRef {
					if _, ok := ref.ReferencedContext.(j.SpanContext); ok {
						childof = true
						break OptsLoop
					}
				}
			case opentracing.Tag:
				tag := opt.(opentracing.Tag)
				if tag.Key == j.PostTraceTag.Key && tag.Value == j.PostTraceTag.Value {
					isPostTrace = true
					break OptsLoop
				}
			case opentracing.Tags:
				tags := opt.(opentracing.Tags)
				if tags[j.PostTraceTag.Key] == j.PostTraceTag.Value {
					isPostTrace = true
					break OptsLoop
				}
			default:
			}
		}

		if childof || isPostTrace {
			span = t.Tracer.StartSpan(operationName, opts...)
		} else {
			span = (opentracing.NoopTracer{}).StartSpan(operationName, opts...)
		}
	}
	return span
}
