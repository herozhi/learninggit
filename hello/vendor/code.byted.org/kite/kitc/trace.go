package kitc

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	olog "github.com/opentracing/opentracing-go/log"

	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/metainfo"
	"code.byted.org/kite/endpoint"
	"code.byted.org/kite/kitc/connpool"
	"code.byted.org/kite/kitutil/kiterrno"
	"code.byted.org/trace/trace-client-go"
	kext "code.byted.org/trace/trace-client-go/ext"
	"code.byted.org/trace/trace-client-go/jaeger-client"
	posttrace "code.byted.org/trace/trace-client-go/post-trace"
	tutil "code.byted.org/trace/trace-client-go/utils"
)

var headerTagMap = map[string]string{
	HeaderTransRemoteAddr: string(ext.PeerAddress),
	HeaderTransToCluster:  string(kext.PeerCluster),
	HeaderTransToIDC:      string(kext.PeerIDC),
}

var headerEventMap = map[string]olog.Field{
	HeaderTransPerfTConnStart: kext.EventKindConnectStart,
	HeaderTransPerfTConnEnd:   kext.EventKindConnectEnd,
	HeaderTransPerfTSendStart: kext.EventKindPkgSendStart,
	HeaderTransPerfTRecvStart: kext.EventKindPkgRecvStart,
	HeaderTransPerfTRecvEnd:   kext.EventKindPkgRecvEnd,
}

var (
	spanEventKindTransportOpenEnum = kext.EventKindEnum("perfT.TransportOpen")
	spanEventKindTransportOpen     = olog.String(string(kext.EventKind), string(spanEventKindTransportOpenEnum))

	spanEventKindTransportCloseEnum = kext.EventKindEnum("perfT.TransportClose")
	spanEventKindTransportClose     = olog.String(string(kext.EventKind), string(spanEventKindTransportCloseEnum))

	spanEventKindTransportFlushEnum = kext.EventKindEnum("perfT.TransportFlush")
	spanEventKindTransportFlush     = olog.String(string(kext.EventKind), string(spanEventKindTransportFlushEnum))

	spanTagBufferedTransport = opentracing.Tag{Key: "transport", Value: "buffered"}
	spanTagFramedTransport   = opentracing.Tag{Key: "transport", Value: "framed"}
)

func fillSpanDataBeforeCall(ctx context.Context, rpcInfo *rpcInfo) {
	span := opentracing.SpanFromContext(ctx)
	if span == nil || !trace.IsSampled(span) {
		return
	}

	ext.SpanKindRPCClient.Set(span)
	ext.PeerService.Set(span, trace.FormatOperationName(rpcInfo.To, rpcInfo.Method))
	ext.Component.Set(span, "kite")
	kext.RPCLogID.Set(span, rpcInfo.LogID)
	kext.LocalCluster.Set(span, env.Cluster())
	kext.LocalIDC.Set(span, env.IDC())
	kext.LocalAddress.Set(span, env.HostIP())
	if rpcInfo.StressTag != "" {
		kext.StressTag.Set(span, rpcInfo.StressTag)
	}
}

func fillSpanDataAfterCall(ctx context.Context, rpcInfo *rpcInfo, resp interface{}, err error) (finishOpts opentracing.FinishOptions) {
	span := opentracing.SpanFromContext(ctx)
	if span == nil || !trace.IsSampled(span) {
		return
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

	if !ServiceMeshMode {
		if ins := rpcInfo.TargetInstance(); ins != nil {
			if port, err := strconv.Atoi(ins.Port); err == nil {
				ext.PeerPort.Set(span, uint16(port))
			}
			if ip := net.ParseIP(ins.Host); ip.To4() == nil {
				ext.PeerHostIPv6.Set(span, ins.Host)
			} else {
				ext.PeerHostIPv4.Set(span, tutil.InetAtoN(ins.Host))
			}
		}
		if rpcInfo.ToCluster != "" {
			kext.PeerCluster.Set(span, rpcInfo.ToCluster)
		}
		if idc := rpcInfo.TargetIDC(); idc != "" {
			kext.PeerIDC.Set(span, idc)
		}
	} else {
		span.SetTag("meshMode", true)
		finishOpts = fillSpanFromProxy(ctx, span)
	}

	// kite-related error may cause rpcInfo.Conn concurrent read-write race condition (eg. RPCTimeout)
	if _, ok := err.(kiterrno.KiteError); !ok {
		if conn, ok := rpcInfo.Conn().(*connpool.ConnWithPkgSize); ok {
			kext.RequestLength.Set(span, atomic.LoadInt32(&(conn.Written)))
			kext.ResponseLength.Set(span, atomic.LoadInt32(&(conn.Readn)))
		}
	}
	return
}

func injectTraceIntoExtra(req interface{}, ctx context.Context) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("%v", e)
		}
	}()

	span := opentracing.SpanFromContext(ctx)
	if trace.IsSampled(span) {
		spanCtx := opentracing.SpanFromContext(ctx).Context()
		carrier := opentracing.TextMapCarrier{}
		if err = opentracing.GlobalTracer().Inject(spanCtx,
			opentracing.TextMap, carrier); err != nil {
			return
		}
		extra := make(map[string]string)
		err = carrier.ForeachKey(func(key, val string) error {
			extra[key] = val
			return nil
		})
		_ = hackExtra(req, extra)
	}

	return
}

type extraBase interface {
	GetExtra() map[string]string
}

func fillPostSpanAfterCall(ctx context.Context, rpcInfo *rpcInfo, resp interface{}, respErr error, begin time.Time) {
	// sampled trace no need tail based
	span := opentracing.SpanFromContext(ctx)
	if span != nil && trace.IsSampled(span) {
		return
	}
	// no init ctx with post trace record
	recorder := posttrace.PostTraceRecorderFromContext(ctx)
	if recorder == nil {
		return
	}
	startopts := make([]opentracing.StartSpanOption, 0)

	// err exist & not ignore err
	needTailBase := respErr != nil && !IsPostTraceIgnoreErrno(respErr)

	// check child span ctx
	if rsp, ok := resp.(endpoint.KiteResponse); ok {
		if extraBase, ok := rsp.GetBaseResp().(extraBase); ok {
			if spanCtxId, ok := extraBase.GetExtra()[jaeger.TraceContextHeaderName]; ok && len(spanCtxId) > 0 {
				spanCtx, err := SpanCtxFromTextMap(extraBase.GetExtra())
				if err == nil {
					startopts = append(startopts, opentracing.SpanReference{
						Type:              posttrace.ParentOfRef,
						ReferencedContext: spanCtx,
					})
				}
				needTailBase = true
			}
		}
	}

	if needTailBase {
		var finishOpts opentracing.FinishOptions
		kvtags := make(map[string]interface{})
		if !ServiceMeshMode {
			if ins := rpcInfo.TargetInstance(); ins != nil {
				if port, err := strconv.Atoi(ins.Port); err == nil {
					kvtags[string(ext.PeerPort)] = uint16(port)
				}
				if ip := net.ParseIP(ins.Host); ip.To4() == nil {
					kvtags[string(ext.PeerHostIPv6)] = ins.Host
				} else {
					kvtags[string(ext.PeerHostIPv4)] = tutil.InetAtoN(ins.Host)
				}
			}
		} else {
			finishOpts, kvtags = fillSpanDataFromProxy(ctx)
			kvtags["meshMode"] = true
		}

		peerOperation := trace.FormatOperationName(rpcInfo.To, rpcInfo.Method)
		kvtags[string(ext.Component)] = "kite"
		kvtags[ext.SpanKindRPCClient.Key] = ext.SpanKindRPCClient.Value
		kvtags[string(ext.PeerService)] = peerOperation

		hasErrorCode, hasStatusCode, statusCode, errorCode, errorType := GetResultCodes(respErr, resp)

		if hasStatusCode {
			kvtags[string(kext.ReturnCode)] = int32(statusCode)
		}
		if hasErrorCode {
			kvtags[string(kext.ErrorCode)] = int32(errorCode)
			kvtags[string(kext.ErrorType)] = int32(errorType)
		}

		if rpcInfo.ToCluster != "" {
			kvtags[string(kext.PeerCluster)] = rpcInfo.ToCluster
		}
		if idc := rpcInfo.TargetIDC(); idc != "" {
			kvtags[string(kext.PeerIDC)] = idc
		}

		// kite-related error may cause rpcInfo.Conn concurrent read-write race condition (eg. RPCTimeout)
		if _, ok := respErr.(kiterrno.KiteError); !ok {
			if conn, ok := rpcInfo.Conn().(*connpool.ConnWithPkgSize); ok {
				kvtags[string(kext.RequestLength)] = atomic.LoadInt32(&(conn.Written))
				kvtags[string(kext.ResponseLength)] = atomic.LoadInt32(&(conn.Readn))
			}
		}
		kvtags[string(ext.Error)] = respErr != nil
		if respErr != nil {
			errLog := olog.String("error.kind", respErr.Error())
			tutil.AppendLogRecord(&finishOpts, &errLog, time.Now())
		}

		startopts = append(startopts, opentracing.StartTime(begin))
		finishOpts.FinishTime = time.Now()
		startopts = append(startopts, opentracing.Tags(kvtags))
		recorder.RecordChild(peerOperation, startopts, finishOpts)
	}
}

func spanStringFromContext(ctx context.Context) (string, bool) {
	span := opentracing.SpanFromContext(ctx)
	if !trace.IsSampled(span) {
		return "", false
	}
	if jSpan, ok := span.(*jaeger.Span); ok {
		return jSpan.String(), true
	}
	return "", false
}

func SpanCtxFromTextMap(extra map[string]string) (opentracing.SpanContext, error) {
	return opentracing.GlobalTracer().Extract(opentracing.TextMap, opentracing.TextMapCarrier(extra))
}

func IsPostTraceIgnoreErrno(err error) bool {
	if errno, ok := err.(kiterrno.KiteError); ok {
		switch errno.Errno() {
		case kiterrno.NotAllowedByServiceCBCode,
			kiterrno.NotAllowedByInstanceCBCode,
			kiterrno.ForbiddenByDegradationCode,
			kiterrno.NotAllowedByACLCode,
			kiterrno.StressBotRejectionCode,
			kiterrno.EndpointQPSLimitRejectCode,
			kiterrno.NotAllowedByUserErrCBCode:
			return true
		default:
			return false
		}
	}
	return false
}

// fill span with info return from proxy
func fillSpanFromProxy(ctx context.Context, span opentracing.Span) opentracing.FinishOptions {
	finishOpts, kvTags := fillSpanDataFromProxy(ctx)
	for k, v := range kvTags {
		span.SetTag(k, v)
	}
	return finishOpts
}

func fillSpanDataFromProxy(ctx context.Context) (opentracing.FinishOptions, map[string]interface{}) {
	var finishOpts opentracing.FinishOptions
	kvtags := make(map[string]interface{})

	for headerKey, tagKey := range headerTagMap {
		if value, ok := metainfo.GetBackwardValue(ctx, headerKey); ok && value != "" {
			kvtags[tagKey] = value
		}
	}
	for headerKey, event := range headerEventMap {
		if timeStamp, ok := metainfo.GetBackwardValue(ctx, headerKey); ok {
			tutil.AppendLogRecordByTimeString(&finishOpts, &event, timeStamp)
		}
	}
	return finishOpts, kvtags
}

// spanTransportOpen
func spanTransportOpen(ctx context.Context, tag opentracing.Tag) {
	span := opentracing.SpanFromContext(ctx)
	if span == nil || !trace.IsSampled(span) {
		return
	}
	tag.Set(span)
	span.LogFields(spanEventKindTransportOpen)
}

// spanTransportClose
func spanTransportClose(ctx context.Context) {
	span := opentracing.SpanFromContext(ctx)
	if span == nil || !trace.IsSampled(span) {
		return
	}
	span.LogFields(spanEventKindTransportClose)
}

// spanTransportFlush
func spanTransportFlush(ctx context.Context) {
	span := opentracing.SpanFromContext(ctx)
	if span == nil || !trace.IsSampled(span) {
		return
	}
	span.LogFields(spanEventKindTransportFlush)
}
