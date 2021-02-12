package ginex

import (
	"code.byted.org/trace/trace-client-go/post-trace"
	"reflect"
	"runtime"
	"strings"

	"code.byted.org/gin/ginex/internal"
	"code.byted.org/trace/trace-client-go"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
)

// Wraps用于解决handler被decorator修饰时不能打出handler名称的问题
// 使用方法见wraps_test.go
type Wraps struct {
	wrapped     interface{}
	handler     gin.HandlerFunc
	handlerName string
}

func NewWraps(wrapped interface{}, handler gin.HandlerFunc) *Wraps {
	wraps := &Wraps{
		wrapped: wrapped,
		handler: handler,
	}
	method := runtime.FuncForPC(reflect.ValueOf(wrapped).Pointer()).Name()
	pos := strings.LastIndexByte(method, '.')
	if pos != -1 {
		method = method[pos+1:]
	}
	wraps.handlerName = method
	return wraps
}

func (w *Wraps) Wrap(ctx *gin.Context) {
	if appConfig.EnableTracing {
		span := opentracing.SpanFromContext(CacheRPCContext(ctx))
		normMethod := trace.FormatOperationName(PSM(), w.handlerName)
		trace.SetOperationName(span, normMethod)
		postTraceRec := posttrace.PostTraceRecorderFromContext(CacheRPCContext(ctx))
		if recorder, ok := postTraceRec.(*posttrace.PostTraceRecorderImpl); ok {
			recorder.SetOperationName(normMethod)
		}
	}
	ctx.Set(internal.METHODKEY, w.handlerName)
	amendCacheCtxHandlerName(ctx, w.handlerName)
	w.handler(ctx)
}
