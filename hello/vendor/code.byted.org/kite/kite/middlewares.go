package kite

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"runtime"
	"strconv"
	"time"

	"github.com/opentracing/opentracing-go"

	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/logid"
	"code.byted.org/gopkg/logs"
	"code.byted.org/gopkg/metainfo"
	"code.byted.org/gopkg/net2"
	"code.byted.org/gopkg/thrift"
	"code.byted.org/kite/endpoint"
	"code.byted.org/kite/kitc"
	"code.byted.org/kite/kitutil"
	"code.byted.org/kite/kitutil/kiterrno"
	"code.byted.org/trace/trace-client-go"
	posttrace "code.byted.org/trace/trace-client-go/post-trace"
)

const (
	defaultAccessLogLocation = "access.go:0"
	defaultStringBuilderSize = 200
)

/* NOTE:
 * AccessLogMW print access log,
 * Close this logger: set "DisableAccessLog: false" in yml
 * AccessLogMW use gopkg/logs.Logger and dont support replace
 */
func AccessLogMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		// NOTE: logger closed
		if DisableAccessLog || accessLogger == nil {
			return next(ctx, request)
		}

		begin := time.Now()
		response, err := next(ctx, request)
		cost := time.Since(begin).Nanoseconds() / 1000 // us

		// Error code and status code are now different
		statusCode, hasStatusCode := kiterrno.GetErrCode(response)
		hasErrorCode, errorCode, errorType := false, 0, 0
		if kerr, ok := err.(kiterrno.KiteError); ok {
			errorCode, errorType = kerr.Errno(), kerr.Category()
			hasErrorCode = true
		}

		r := GetRPCInfo(ctx)
		// NOTE: 构建 tags
		kvs := []interface{}{
			"method", r.Method,
			"rip", empty2Default(r.RemoteIP, "-"),
			"called", empty2Default(r.UpstreamService, "-"),
			"from_cluster", empty2Default(r.UpstreamCluster, "-"),
			"cost", strconv.FormatInt(cost, 10),
			"env", empty2Default(r.Env, "-"),
			"mesh", ifelse(ServiceMeshMode, "1", "0"),
		}
		if r.StressTag != "" {
			kvs = append(kvs, "stress_tag", r.StressTag)
		}
		if hasErrorCode {
			kvs = append(kvs, "error_code", errorCode, "error_type", errorType)
		}
		if hasStatusCode {
			kvs = append(kvs, "status", statusCode)
		}

		stats, ok := getRPCStats(ctx)
		if ok {
			// delay logging to get extra information
			stats.AddCallback(func(ss *rpcStats) {
				kvs = append(kvs,
					"read_size", ss.ReadSize(),
					"write_size", ss.WriteSize(),
					"read_cost", ss.ReadCostUS(),
					"write_cost", ss.WriteCostUS(),
					"process_cost", ss.ProcessCostUS(),
					"total_cost", ss.TotalCostUS(),
				)
				if err != nil {
					accessLogger.CtxErrorflKVs(defaultAccessLogLocation, ctx, kvs...)
				} else {
					accessLogger.CtxTraceflKVs(defaultAccessLogLocation, ctx, kvs...)
				}
			})
		} else {
			if err != nil {
				accessLogger.CtxErrorflKVs(defaultAccessLogLocation, ctx, kvs...)
			} else {
				accessLogger.CtxTraceflKVs(defaultAccessLogLocation, ctx, kvs...)
			}
		}

		return response, err
	}
}

const (
	recoverMW = "RecoverMW"
)

// RecoverMW print panic info to
func RecoverMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		defer func() {
			if e := recover(); e != nil {
				const size = 64 << 10
				buf := make([]byte, size)
				buf = buf[:runtime.Stack(buf, false)]
				logs.CtxError(ctx, "KITE: panic in handler: %s: %s", e, buf)
				panic(recoverMW)
			}
		}()
		return next(ctx, request)
	}
}

// PushNoticeMW .
func PushNoticeMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		ctx = logs.NewNoticeCtx(ctx)
		defer logs.CtxFlushNotice(ctx)
		return next(ctx, request)
	}
}

// BaseRespCheckMW .
func BaseRespCheckMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		resp, err := next(ctx, request)
		if err != nil {
			if _, ok := err.(kiterrno.KiteError); !ok {
				// Wrap error to be KiteError
				return nil, kiterrno.NewKiteError(kiterrno.UNREGISTERED, 0, err)
			}
			return nil, err
		}

		r := GetRPCInfo(ctx)
		response, ok := resp.(endpoint.KiteResponse)
		if !ok {
			panic(fmt.Sprintf("response type error in %s's %s method. The error type is %s.",
				r.Service, r.Method, reflect.TypeOf(resp)))
		}
		if response.GetBaseResp() == nil {
			panic(fmt.Sprintf("response's KiteBaseResp is nil in %s's %s method.", r.Service, r.Method))
		}
		return response, nil
	}
}

// Mesh THeader set reply Headers .
func MeshReplyHeadersMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		resp, err := next(ctx, request)
		rpcInfo := GetRPCInfo(ctx)
		tp, ok := ctx.Value(infrastructionKey).(thrift.TProtocol)
		if rpcInfo.httpContentType != "" && ok {
			trans := tp.Transport().(*thrift.HeaderTransport)
			trans.SetIntHeader(kitc.HTTP_CONTENT_TYPE, rpcInfo.httpContentType)
		}
		return resp, err
	}
}

type extraBase interface {
	GetExtra() map[string]string
}

// ParserMW init rpcinfo
func ParserMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		rpcInfo := GetRPCInfo(ctx)
		rpcInfo.Service = ServiceName
		rpcInfo.Cluster = ServiceCluster
		// 1. read rpcInfo from base
		if r, ok := request.(endpoint.KiteRequest); ok && r.IsSetBase() {
			b := r.GetBase()
			if extra, ok := b.(extraBase); ok {
				rpcInfo.StressTag = extra.GetExtra()["stress_tag"]
				rpcInfo.DDPTag = extra.GetExtra()["ddp_tag"]
				if userExtraStr := extra.GetExtra()["user_extra"]; len(userExtraStr) > 0 {
					ue, err := kitutil.JSONStr2Map(userExtraStr)
					if err != nil {
						logs.CtxError(ctx, "unmarshal user extra map err: %s", err.Error())
					} else {
						ctx = kitutil.NewCtxWithUpstreamUserExtra(ctx, ue)
						ctx = metainfo.SetMetaInfoFromMap(ctx, ue)
					}
				}
			}

			rpcInfo.UpstreamService = b.GetCaller()
			rpcInfo.UpstreamCluster = b.GetCluster()
			rpcInfo.Env = b.GetEnv()
			rpcInfo.Client = b.GetClient()
			rpcInfo.RemoteIP = b.GetAddr()
			rpcInfo.LogID = b.GetLogID()
		}

		// 2. read rpcInfo from ttheader, ttheader takes precedence over base
		var headers map[string]string
		prot, ok := ctx.Value(infrastructionKey).(thrift.TProtocol)
		if ok {
			if hprot, ok := prot.Transport().(*thrift.HeaderTransport); ok {
				// Get headers from protocol
				headers = hprot.ReadHeaders()
				ctx = metainfo.SetMetaInfoFromMap(ctx, headers)

				if val, ok := hprot.ReadIntHeader(ikFromIDC); ok && len(val) > 0 {
					rpcInfo.UpstreamIDC = val
				}
				if val, ok := hprot.ReadIntHeader(ikFromService); ok && len(val) > 0 {
					rpcInfo.UpstreamService = val
				}
				if val, ok := hprot.ReadIntHeader(ikEnv); ok && len(val) > 0 {
					rpcInfo.Env = val
				}
				if val, ok := hprot.ReadIntHeader(ikLogID); ok && len(val) > 0 {
					rpcInfo.LogID = val
				}
				if val, ok := hprot.ReadIntHeader(ikStressTag); ok && len(val) > 0 {
					rpcInfo.StressTag = val
				}
			}
		}

		ctx = metainfo.TransferForward(ctx)

		if GetRealIP {
			if addr, ok := ctx.Value(addrFromConnection).(string); ok {
				rpcInfo.RemoteIP, _, _ = net.SplitHostPort(addr)
			}
			if headers["rip"] != "" {
				rpcInfo.RemoteIP, _, _ = net.SplitHostPort(headers["rip"])
			}
		}

		// 3. fallback
		// 如果 RemoteIP 是 ipv4，那么这里打点为 ipv4 地址
		// 否则使用 ipv6 地址
		rpcInfo.LocalIP = env.HostIP()
		// 这里有个隐含条件：远端是 ipv6 的话，本地必有 ipv6
		if rpcInfo.RemoteIP != "" && net2.IsV6IP(net.ParseIP(rpcInfo.RemoteIP)) {
			rpcInfo.LocalIP = env.HostIPV6()
		}

		if rpcInfo.UpstreamIDC == "" {
			rpcInfo.UpstreamIDC = "none"
		}
		if rpcInfo.UpstreamService == "" || rpcInfo.UpstreamService == "-" {
			rpcInfo.UpstreamService = "none"
		}
		if rpcInfo.UpstreamCluster == "" || rpcInfo.UpstreamCluster == "-" {
			rpcInfo.UpstreamCluster = "default"
		}

		if rpcInfo.DDPTag != "" {
			ctx = kitutil.NewCtxWithDDPRoutingTag(ctx, rpcInfo.DDPTag)
		}

		if rpcInfo.LogID == "" {
			rpcInfo.LogID = logid.GenLogID()
		}

		rpcInfo.RPCConfig = RPCServer.getRPCConfig(rpcInfo.RPCMeta)
		return next(ctx, request)
	}
}

// ExposeCtxMW expose some variable from RPCInfo to the context for compatibility
func ExposeCtxMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		r := GetRPCInfo(ctx)
		ctx = kitutil.NewCtxWithServiceName(ctx, r.Service)
		ctx = kitutil.NewCtxWithCluster(ctx, r.Cluster)
		ctx = kitutil.NewCtxWithCaller(ctx, r.UpstreamService)
		ctx = kitutil.NewCtxWithCallerCluster(ctx, r.UpstreamCluster)
		ctx = kitutil.NewCtxWithMethod(ctx, r.Method)
		ctx = kitutil.NewCtxWithLogID(ctx, r.LogID)
		ctx = kitutil.NewCtxWithEnv(ctx, r.Env)
		ctx = kitutil.NewCtxWithClient(ctx, r.Client)
		ctx = kitutil.NewCtxWithLocalIP(ctx, r.LocalIP)
		ctx = kitutil.NewCtxWithAddr(ctx, r.RemoteIP)
		ctx = kitutil.NewCtxWithStressTag(ctx, r.StressTag)
		ctx = kitutil.NewCtxWithTraceTag(ctx, r.TraceTag)
		return next(ctx, request)
	}
}

// ACLMW .
func ACLMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		r := GetRPCInfo(ctx)
		if !r.ACLAllow {
			return nil, kiterrno.NewKiteError(kiterrno.KITE, kiterrno.NotAllowedByACLCode,
				fmt.Errorf("upstream service=%s, cluster=%s", r.UpstreamService, r.UpstreamCluster))
		}
		return next(ctx, request)
	}
}

// StressBotMW .
func StressBotMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		r := GetRPCInfo(ctx)
		if !r.StressBotSwitch && r.StressTag != "" {
			return nil, kiterrno.NewKiteError(kiterrno.KITE, kiterrno.StressBotRejectionCode,
				fmt.Errorf("upstream service=%s, cluster=%s", r.UpstreamService, r.UpstreamCluster))
		}
		return next(ctx, request)
	}
}

// EndpointQPSLimitMW control the traffic on Endpoint
func EndpointQPSLimitMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		r := GetRPCInfo(ctx)

		if !RPCServer.overloader.TakeEndpointQPS(r.Method) {
			return nil, kiterrno.NewKiteError(kiterrno.KITE, kiterrno.EndpointQPSLimitRejectCode,
				fmt.Errorf("service=%s, cluster=%s method=%s", r.Service, r.Cluster, r.Method))
		}

		return next(ctx, request)
	}
}

// AdditionMW .
func AdditionMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		r := GetRPCInfo(ctx)
		method := r.Method
		if method == "" {
			method = "-"
		}
		if mw, ok := mMap[method]; ok {
			next = mw(next)
		}
		if userMW != nil {
			next = userMW(next)
		}
		return next(ctx, request)
	}
}

// OpentracingMW
func OpenTracingMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		rpcInfo := GetRPCInfo(ctx)
		normOperation := trace.FormatOperationName(rpcInfo.Service, rpcInfo.Method)
		if r, ok := request.(endpoint.KiteRequest); ok && r.IsSetBase() {
			b := r.GetBase()
			if extra, ok := b.(extraBase); ok {
				var span opentracing.Span
				if spanCtx, err := kitc.SpanCtxFromTextMap(extra.GetExtra()); err == nil {
					// logs.Info("span-ctx: %s from client-side", trace.JSpanContextToString(spanCtx))
					span = opentracing.StartSpan(normOperation, opentracing.ChildOf(spanCtx))
				} else {
					span = opentracing.StartSpan(normOperation)
				}
				// finishing span. opentracing.StartSpan should not return nil object
				defer span.Finish()
				ctx = opentracing.ContextWithSpan(ctx, span)
				if EnableDyeLog && trace.IsDebug(span) {
					ctx = context.WithValue(ctx, logs.DynamicLogLevelKey, DyeLogLevel)
				}
				rpcInfo.TraceTag = trace.JSpanContextToString(span.Context())
			}
		}

		fillSpanDataBeforeHandler(ctx, rpcInfo)
		ctx = posttrace.ContextWithPostTraceRecorder(ctx, normOperation, false)
		resp, err := next(ctx, request)
		fillSpanDataAfterHandler(ctx, rpcInfo, resp, err)
		reportPostSpansAfterHandler(ctx, rpcInfo, resp, err)
		return resp, err
	}
}

func hackExtra(resp interface{}, kvs map[string]string) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("%v", e)
		}
	}()

	ptr := reflect.ValueOf(resp)
	for ptr.Kind() == reflect.Ptr {
		ptr = ptr.Elem()
	}
	if ptr.Kind() != reflect.Struct {
		return nil
	}
	// get BaseResp
	basePtr := ptr.FieldByName("BaseResp")
	for basePtr.Kind() == reflect.Ptr {
		basePtr = basePtr.Elem()
	}
	if basePtr.Kind() != reflect.Struct {
		return nil
	}

	// get Extra
	extra := basePtr.FieldByName("Extra")
	if !extra.CanSet() {
		return nil
	}
	origin, ok := extra.Interface().(map[string]string)
	if !ok {
		return fmt.Errorf("kite set BaseResp.Extra failed")
	}
	for k := range origin {
		if _, ok := kvs[k]; !ok {
			kvs[k] = origin[k]
		}
	}
	extra.Set(reflect.ValueOf(kvs))
	return nil
}
