package posttrace

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	trace "code.byted.org/trace/trace-client-go"
	kext "code.byted.org/trace/trace-client-go/ext"
	j "code.byted.org/trace/trace-client-go/jaeger-client"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
)

// StartSpanWithPostTrace return ctx, span and post trace recorder after start span along with post trace
func StartSpanWithPostTrace(ctx context.Context, isRoot bool, operationName string, opts ...opentracing.StartSpanOption) (context.Context, opentracing.Span, *PostTraceRecorderImpl) {
	span := opentracing.StartSpan(operationName, opts...)
	ctx = opentracing.ContextWithSpan(ctx, span)

	if span != nil && trace.IsSampled(span) {
		return ctx, span, nil
	}

	r := NewPostTraceRecorderImpl(operationName, isRoot)

	ctx = context.WithValue(ctx, postTraceRecorderCtxKey, r)
	return ctx, span, r
}

// StartSpanWithStartTimeAndPostTrace return ctx, span and post trace recorder after start span along with post trace
func StartSpanWithStartTimeAndPostTrace(ctx context.Context, startTime time.Time, isRoot bool, operationName string, opts ...opentracing.StartSpanOption) (context.Context, opentracing.Span, *PostTraceRecorderImpl) {
	opts = append(opts, opentracing.StartTime(startTime))
	span := opentracing.StartSpan(operationName, opts...)
	ctx = opentracing.ContextWithSpan(ctx, span)

	if span != nil && trace.IsSampled(span) {
		return ctx, span, nil
	}

	r := NewPostTraceRecorderImplWithStartTime(operationName, isRoot, startTime)

	ctx = context.WithValue(ctx, postTraceRecorderCtxKey, r)
	return ctx, span, r
}

// ContextWithPostTraceRecorder is deprecated !!!!!!!!!!!!!!
// ContextWithPostTraceRecorder returns a new `context.Context` that holds a reference to
// a PostTraceRecorder.
// isRoot: indicate if start a root span or not
func ContextWithPostTraceRecorder(ctx context.Context, operationName string, isRoot bool) context.Context {
	span := opentracing.SpanFromContext(ctx)
	if span != nil && trace.IsSampled(span) {
		return ctx
	}

	rec := NewPostTraceRecorderImpl(operationName, isRoot)
	return context.WithValue(ctx, postTraceRecorderCtxKey, rec)
}

type PostTraceRecorderImpl struct {
	throttled     bool
	isRoot        bool
	startTime     time.Time
	needReported  int32
	operationName string
	root          PostTraceSpanRecord
	children      []PostTraceSpanRecord
	mu            sync.RWMutex
}

// NewPostTraceRecorderImpl alloc and init a PostTraceRecorder object
func NewPostTraceRecorderImpl(operationName string, isRoot bool) *PostTraceRecorderImpl {
	r := new(PostTraceRecorderImpl)
	r.operationName = operationName
	r.isRoot = isRoot
	r.startTime = time.Now()
	return r
}

// NewPostTraceRecorderImplWithStartTime alloc and init a PostTraceRecorder object
func NewPostTraceRecorderImplWithStartTime(operationName string, isRoot bool, startTime time.Time) *PostTraceRecorderImpl {
	r := new(PostTraceRecorderImpl)
	r.operationName = operationName
	r.isRoot = isRoot
	r.startTime = startTime
	return r
}

type PostTraceSpanRecord struct {
	StartOpts  []opentracing.StartSpanOption
	FinishOpts opentracing.FinishOptions
}

// Start is deprecated !!!!!!!!!!!!!!
func (r *PostTraceRecorderImpl) Start(operationName string, isRoot bool) {
	r.mu.Lock()
	r.operationName = operationName
	r.isRoot = isRoot
	r.startTime = time.Now()
	r.mu.Unlock()
}

func (r *PostTraceRecorderImpl) SetOperationName(operationName string) {
	r.mu.Lock()
	r.operationName = operationName
	r.mu.Unlock()
}

// Replay is deprecated !!!!!!!!!!!!!!
// Replay report all client spans recorded before and return the parent span of them, with its FinishOptions.
// outside should finish the returned span with returned options if the returned span != nil
func (r *PostTraceRecorderImpl) Replay() (opentracing.Span, opentracing.FinishOptions) {
	if atomic.LoadInt32(&r.needReported) == 0 {
		return nil, r.root.FinishOpts
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.needReported == 0 {
		return nil, r.root.FinishOpts
	}

	return r.replayNoLocking()
}

// ReplayV2 report all client spans recorded before and return the parent span of them, with its FinishOptions.
// outside should finish the returned span with returned options if the returned span != nil
func (r *PostTraceRecorderImpl) ReplayV2(logID string) (opentracing.Span, opentracing.FinishOptions) {
	if atomic.LoadInt32(&r.needReported) == 0 {
		return nil, r.root.FinishOpts
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.needReported == 0 {
		return nil, r.root.FinishOpts
	}

	r.root.StartOpts = append(r.root.StartOpts, opentracing.Tag{Key: string(kext.RPCLogID), Value: logID})

	return r.replayNoLocking()
}

func (r *PostTraceRecorderImpl) replayNoLocking() (opentracing.Span, opentracing.FinishOptions) {

	r.root.StartOpts = append(r.root.StartOpts, j.PostTraceTag, opentracing.StartTime(r.startTime))
	if r.isRoot {
		r.root.StartOpts = append(r.root.StartOpts, j.PostTraceRootSpanTag)
	}

	// server span
	span := opentracing.StartSpan(r.operationName, r.root.StartOpts...)

	if !trace.IsSampled(span) {
		// sth wrong inside StartSpan, break here
		return span, r.root.FinishOpts
	}

	span = span.SetTag("throttled", r.throttled)

	ctx := span.Context()

	// all clients spans
	for _, child := range r.children {
		opts := child.StartOpts
		opts = append(opts, opentracing.ChildOf(ctx), j.PostTraceTag)
		childSpan := opentracing.StartSpan(r.operationName, opts...)
		childSpan.FinishWithOptions(child.FinishOpts)
	}

	return span, r.root.FinishOpts
}

func (r *PostTraceRecorderImpl) RecordNeedReported() {
	r.mu.Lock()
	r.setNeedReportedNoLocking()
	r.mu.Unlock()
}

func (r *PostTraceRecorderImpl) RecordTag(key string, value interface{}) {
	r.mu.Lock()
	r.root.StartOpts = append(r.root.StartOpts, opentracing.Tag{key, value})
	r.mu.Unlock()
}

func (r *PostTraceRecorderImpl) RecordLogFields(fields ...log.Field) {
	r.mu.Lock()
	r.root.FinishOpts.LogRecords = append(r.root.FinishOpts.LogRecords, opentracing.LogRecord{
		Fields:    fields,
		Timestamp: time.Now(),
	})
	r.mu.Unlock()
}

func (r *PostTraceRecorderImpl) RecordChild(peerService string, startOpts []opentracing.StartSpanOption, finishOpts opentracing.FinishOptions) {
	r.mu.Lock()

	defer r.mu.Unlock()

	if trace.PostTraceIsBlock(r.operationName, peerService) {
		r.throttled = true
		return
	}

	r.children = append(r.children, PostTraceSpanRecord{StartOpts: startOpts, FinishOpts: finishOpts})
	r.setNeedReportedNoLocking()

}

func (r *PostTraceRecorderImpl) Children() []PostTraceSpanRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.children
}

func (r *PostTraceRecorderImpl) setNeedReportedNoLocking() {
	if r.needReported == 1 {
		return
	}

	if trace.PostTraceIsBlock(r.operationName, "") {
		return
	}

	r.needReported = 1
}
