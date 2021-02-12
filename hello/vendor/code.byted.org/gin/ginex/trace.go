package ginex

import (
	"net/http"
	"strconv"

	"code.byted.org/gin/ginex/internal"
	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/logs"
	trace "code.byted.org/trace/trace-client-go"
	kext "code.byted.org/trace/trace-client-go/ext"
	posttrace "code.byted.org/trace/trace-client-go/post-trace"
	"github.com/gin-gonic/gin"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

const postTraceRecorderCtxKey = "PostTraceRecorder"

func OpentracingHandler() gin.HandlerFunc {
	return func(ginCtx *gin.Context) {
		val, exist := ginCtx.Get(internal.SPANCTXKEY)
		span, ok := val.(opentracing.Span)
		if !exist || !ok || span == nil {
			ginCtx.Next()
			return
		}

		ginCtx.Next()

		if trace.IsSampled(span) {
			if logID, exist := ginCtx.Get(internal.LOGIDKEY); exist {
				kext.RPCLogID.Set(span, logID.(string))
			}
			m := getGinCtxMutex(ginCtx)
			m.Lock()
			fillSpanAfterHandler(span, ginCtx)
			m.Unlock()
		} else {
			reportPostSpansAfterHandler(ginCtx)
		}
	}
}

func DyeForceTraceHandler() gin.HandlerFunc {
	return func(ginCtx *gin.Context) {
		// init tracing root span if the trace is enabled
		operation := "-"
		// cause wraps.go::Wrap all registed api-funcs will return the same "Wrap-fn" here.
		// so wraps.go::Wrap need to call SetOperationName to fill real method name into Span
		if method, exist := ginCtx.Get(internal.METHODKEY); exist {
			operation = method.(string)
		}
		normOperation := trace.FormatOperationName(PSM(), operation)
		carrier := opentracing.HTTPHeadersCarrier(ginCtx.Request.Header)
		clientContext, err := opentracing.GlobalTracer().Extract(opentracing.HTTPHeaders, carrier)
		var span opentracing.Span
		if err == nil {
			span = opentracing.GlobalTracer().StartSpan(normOperation, opentracing.ChildOf(clientContext))
		} else {
			span = opentracing.GlobalTracer().StartSpan(normOperation)
		}
		defer func() {
			if span != nil {
				span.Finish()
			}
		}()
		rec := new(posttrace.PostTraceRecorderImpl)
		if !trace.IsSampled(span) {
			rec.Start(normOperation, true)
			ginCtx.Set(postTraceRecorderCtxKey, rec)
		}

		ginCtx.Set(internal.SPANCTXKEY, span)

		if span != nil {
			var deviceID int64 = 0
			var appid int64 = 0
			var hasTraceTag bool
			var err error
			var ttTraceTagVal interface{}

			q := ginCtx.Request.URL.Query()
			if vs, exist := q[appConfig.DeviceIDParamKey]; exist && len(vs) > 0 {
				if deviceID, err = strconv.ParseInt(vs[0], 10, 64); err != nil {
					deviceID = 0
				}
			}

			if vs, exist := q[appConfig.AppIDParamKey]; exist && len(vs) > 0 {
				if appid, err = strconv.ParseInt(vs[0], 10, 64); err != nil {
					appid = 0
				}
			}

			if trace.AllowTraceTagFromTLB() {
				ttTraceTagVal, hasTraceTag = ginCtx.Get(internal.TT_TRACE_TAG)
			}

			if hasTraceTag ||
				(deviceID != 0 &&
					(trace.DyeUUID(deviceID) ||
						(appid != 0 && trace.DyeAppUUID(int32(appid), deviceID)))) {
				trace.ForceTrace(span)
				if appConfig.EnableDyeLog {
					ginCtx.Set(logs.DynamicLogLevelKey, appConfig.DyeLogLevel)
				}
			}

			if hasTraceTag && ttTraceTagVal != nil {
				span.SetTag("http.x-tt-trace", ttTraceTagVal)
			}

			if deviceID != 0 {
				span.SetTag("device_id", deviceID)
				rec.RecordTag("device_id", deviceID)
			}

			if appid != 0 {
				span.SetTag("app_id", appid)
				rec.RecordTag("app_id", appid)
			}
		}

		ginCtx.Next()
	}
}

func fillSpanAfterHandler(span opentracing.Span, ginCtx *gin.Context) {
	ext.SpanKindRPCServer.Set(span)
	ext.Component.Set(span, "ginex")
	if ginCtx.Request != nil && ginCtx.Request.URL != nil {
		ext.HTTPMethod.Set(span, ginCtx.Request.Method)
		ext.HTTPUrl.Set(span, ginCtx.Request.URL.Path)
		kext.HTTPParam.Set(span, ginCtx.Request.URL.RawQuery)
	}

	kext.LocalAddress.Set(span, LocalIP())
	kext.LocalCluster.Set(span, LocalCluster())
	kext.LocalIDC.Set(span, env.IDC())
	kext.LocalPodName.Set(span, env.PodName())
	kext.LocalEnv.Set(span, ginCtx.GetString(internal.ENVKEY))
	kext.HTTPHost.Set(span, ginCtx.Request.Host)
	stressTag := GetGinCtxStressTag(ginCtx)
	if stressTag != "" {
		kext.StressTag.Set(span, stressTag)
	}
	statusCode := ginCtx.Writer.Status()
	ext.HTTPStatusCode.Set(span, uint16(statusCode))
	if statusCode < http.StatusOK || statusCode >= http.StatusBadRequest {
		ext.Error.Set(span, true)
	}
	fillHTTPHeaders(span, ginCtx)
	kext.ResponseLength.Set(span, int32(ginCtx.Writer.Size()))
}

func fillHTTPHeaders(span opentracing.Span, ginCtx *gin.Context) {
	if ginCtx == nil {
		return
	}
	header := ginCtx.Writer.Header()
	if code, err := strconv.Atoi(header.Get("tt_stable")); err == nil {
		kext.HTTPHeaderTTStable.Set(span, int32(code))
	}
	if code, err := strconv.Atoi(header.Get("bd-tt-error-code")); err == nil {
		kext.HTTPHeaderBdTtErrorCode.Set(span, int32(code))
	}
}

func reportPostSpansAfterHandler(ginCtx *gin.Context) {
	if ginCtx == nil {
		return
	}
	if rec, exist := ginCtx.Get(postTraceRecorderCtxKey); exist {
		if recorder, ok := rec.(*posttrace.PostTraceRecorderImpl); ok {
			if logID, exist := ginCtx.Get(internal.LOGIDKEY); exist {
				recorder.RecordTag(string(kext.RPCLogID), logID.(string))
			}

			// replay client span report
			svrSpan, finishOptions := recorder.Replay()
			if svrSpan == nil {
				return
			}
			if trace.IsSampled(svrSpan) {
				fillSpanAfterHandler(svrSpan, ginCtx)
			}
			// if/not sampled both report svr span
			svrSpan.FinishWithOptions(finishOptions)
		}
	}
}
