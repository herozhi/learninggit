package kite

import (
	"context"
	"net"

	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/logs"
	"code.byted.org/kite/endpoint"
	"code.byted.org/kite/kitc"
	"code.byted.org/trace/trace-client-go"
	kext "code.byted.org/trace/trace-client-go/ext"
	"code.byted.org/trace/trace-client-go/jaeger-client"
	posttrace "code.byted.org/trace/trace-client-go/post-trace"
	tutil "code.byted.org/trace/trace-client-go/utils"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	olog "github.com/opentracing/opentracing-go/log"
)

func fillSpanDataBeforeHandler(ctx context.Context, rpcInfo *RPCInfo) {
	span := opentracing.SpanFromContext(ctx)
	if span == nil || !trace.IsSampled(span) {
		return
	}
	kext.RPCLogID.Set(span, rpcInfo.LogID)
	return
}

func fillSpanDataAfterHandler(ctx context.Context, rpcInfo *RPCInfo, resp interface{}, err error) {
	span := opentracing.SpanFromContext(ctx)
	if span == nil || !trace.IsSampled(span) {
		return
	}

	ext.Component.Set(span, "kite")
	ext.SpanKindRPCServer.Set(span)
	ext.PeerService.Set(span, trace.FormatOperationName(rpcInfo.UpstreamService, "-"))
	if ip := net.ParseIP(rpcInfo.RemoteIP); ip.To4() == nil {
		ext.PeerHostIPv6.Set(span, rpcInfo.RemoteIP)
	} else {
		ext.PeerHostIPv4.Set(span, tutil.InetAtoN(rpcInfo.RemoteIP))
	}
	kext.PeerCluster.Set(span, rpcInfo.UpstreamCluster)

	kext.LocalIDC.Set(span, env.IDC())
	kext.LocalCluster.Set(span, env.Cluster())
	kext.LocalAddress.Set(span, rpcInfo.LocalIP)
	kext.LocalPodName.Set(span, env.PodName())
	kext.LocalEnv.Set(span, rpcInfo.Env)
	if rpcInfo.StressTag != "" {
		kext.StressTag.Set(span, rpcInfo.StressTag)
	}

	ext.Error.Set(span, err != nil)
	if err != nil {
		span.LogFields(olog.String("error.kind", err.Error()))
	}

	hasErrorCode, hasStatusCode, statusCode, errorCode, errorType := GetResultCodes(err, resp)
	if hasStatusCode {
		kext.ReturnCode.Set(span, int32(statusCode))
	}
	if hasErrorCode {
		kext.ErrorCode.Set(span, int32(errorCode))
		kext.ErrorType.Set(span, int32(errorType))
	}

	if response, ok := resp.(endpoint.KiteResponse); ok {
		if response.GetBaseResp() != nil {
			if response.GetBaseResp().GetStatusCode() != 0 {
				span.LogFields(olog.String("message", response.GetBaseResp().GetStatusMessage()))
			}
		}
	}

	_ = trace.FillSpanEvent(ctx, kext.EventKindPkgSendStartEnum)
}

func reportPostSpansAfterHandler(ctx context.Context, rpcInfo *RPCInfo, resp interface{}, err error) {
	span := opentracing.SpanFromContext(ctx)
	if span != nil && trace.IsSampled(span) {
		return
	}
	// no init ctx with post trace record
	recorder, ok := posttrace.PostTraceRecorderFromContext(ctx).(*posttrace.PostTraceRecorderImpl)
	if !ok || recorder == nil {
		return
	}
	if err != nil && !kitc.IsPostTraceIgnoreErrno(err) {
		recorder.RecordNeedReported()
	}
	// for gen traceid by logid
	recorder.RecordTag(string(kext.RPCLogID), rpcInfo.LogID)
	// replay client span report
	svrSpan, finishOptions := recorder.Replay()
	if svrSpan == nil {
		return
	}
	isSampled := trace.IsSampled(svrSpan)
	if isSampled {
		ctx = opentracing.ContextWithSpan(ctx, svrSpan)
		fillSpanDataAfterHandler(ctx, rpcInfo, resp, err)
	}
	// report svr span
	svrSpan.FinishWithOptions(finishOptions)

	// inject resp.baseResp.extra
	if resp == nil || !isSampled {
		return
	}

	extra := make(map[string]string)
	if span, ok := svrSpan.(*jaeger.Span); ok {
		extra[jaeger.TraceContextHeaderName] = span.String()
	}
	if err = hackExtra(resp, extra); err != nil {
		logs.Debug("inject to baseResp err %v", err)
	}
}
