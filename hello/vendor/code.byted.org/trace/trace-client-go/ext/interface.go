package ext

import (
	opentracing "github.com/opentracing/opentracing-go"
)

// SpanWithSampleFlag .
type SpanWithSampleFlag interface {
	opentracing.Span
	IsSampled() bool
}
