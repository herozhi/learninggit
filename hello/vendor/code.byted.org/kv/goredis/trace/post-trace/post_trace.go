package posttrace

import (
	"context"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
)

type PostTraceRecorder interface {
	RecordTag(key string, value interface{})
	RecordLogFields(fields ...log.Field)
	RecordChild(peerService string, startOpts []opentracing.StartSpanOption, finishOpts opentracing.FinishOptions)
}

const (
	ParentOfRef opentracing.SpanReferenceType = opentracing.ChildOfRef + 100

	postTraceRecorderCtxKey = "PostTraceRecorder"
)

// PostTraceRecorderFromContext returns the `PostTraceRecorder` previously associated with `ctx`, or
// `nil` if no such `PostTraceRecorder` could be found.
func PostTraceRecorderFromContext(ctx context.Context) PostTraceRecorder {
	val := ctx.Value(postTraceRecorderCtxKey)
	if rec, ok := val.(PostTraceRecorder); ok {
		return rec
	}
	return nil
}
