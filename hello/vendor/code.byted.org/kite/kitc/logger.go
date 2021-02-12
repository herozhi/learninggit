package kitc

import (
	"context"
	"reflect"
	"strconv"
	"sync/atomic"
	"time"

	"code.byted.org/gopkg/logs"
	"code.byted.org/kite/endpoint"
	"code.byted.org/kite/kitc/connpool"
	"code.byted.org/kite/kitutil/kiterrno"
)

const (
	defaultKitcLogLocation = "call.go:0"
	localLogVersion        = "v1(6)"
	localLayout            = "2006-01-02 15:04:05.000"
)

// TraceLogger .
type TraceLogger interface {
	Trace(format string, v ...interface{})
	Error(format string, v ...interface{})
}

// SetCallLog which logger for logging calling logs
func SetCallLog(lg TraceLogger) {
	kitcLogger = lg
}

var kitcLogger TraceLogger = &localLogger{}

/*
 * logger 推荐使用 gopkg/logs.Logger, 持续支持 logs 库的性能优化
 * 支持自实现的logger, 只要符合 gopkg/logs/iface.go 中定义的接口规范即可
 *
 * 接口规范:(以下接口规范实现任一即可)
 * [TraceLogger]        最老的接口定义
 * [logs.CtxflKVsLogger] 带 caller 路径的接口定义
 */
// NOTE: logger == nil 表示关闭日志
func NewRPCLogMW(mwCtx context.Context) endpoint.Middleware {
	var logger = kitcLogger

	// NOTE: DisableRPCLog 检测
	if opts, ok := mwCtx.Value(KITC_OPTIONS_KEY).(*Options); ok && opts.DisableRPCLog {
		logger = nil
	}
	// NOTE: 关闭日志, 则直接返回空白中间件
	if logger == nil || reflect.ValueOf(logger).IsNil() {
		return endpoint.EmptyMiddleware
	}

	// NOTE: 输出日志的方式, 根据 logger 类型区分行为
	var pushlog func(ctx context.Context, kvs []interface{}, err error)

	if flKVLogger, ok := logger.(logs.CtxflKVsLogger); ok && flKVLogger != nil {
		// 1. 使用 flKVLogger 输出
		pushlog = func(ctx context.Context, kvs []interface{}, err error) {
			if err != nil {
				flKVLogger.CtxErrorflKVs(defaultKitcLogLocation, ctx, kvs...)
			} else {
				flKVLogger.CtxTraceflKVs(defaultKitcLogLocation, ctx, kvs...)
			}
		}
	} else {
		// 2. 使用 TraceLogger 输出
		pushlog = func(ctx context.Context, kvs []interface{}, err error) {
			rpcInfo := GetRPCInfo(ctx)
			basic := []string{defaultKitcLogLocation, rpcInfo.LocalIP, rpcInfo.From, rpcInfo.LogID, rpcInfo.FromCluster}
			if err != nil {
				logger.Error(formatLog(basic, kvs))
			} else {
				logger.Trace(formatLog(basic, kvs))
			}
		}
	}

	return func(next endpoint.EndPoint) endpoint.EndPoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
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

			rpcInfo := GetRPCInfo(ctx)
			remoteIP := rpcInfo.GetRemoteIP(ctx, response)
			toCluster := ifelse(ServiceMeshMode, rpcInfo.GetToCluster(ctx, response), rpcInfo.ToCluster)

			// NOTE: 构建 tags
			kvs := []interface{}{
				"method", rpcInfo.Method,
				"rip", remoteIP,
				"called", rpcInfo.To,
				"to_cluster", toCluster,
				"from_method", rpcInfo.FromMethod,
				"cost", cost,
				"conn_cost", atomic.LoadInt32(&rpcInfo.ConnCost),
				"env", rpcInfo.Env,
				"mesh", ifelse(ServiceMeshMode, "1", "0"),
			}
			kvs = addStatsKVs(kvs, rpcInfo)
			if rpcInfo.StressTag != "" {
				kvs = append(kvs, "stress_tag", rpcInfo.StressTag)
			}
			if hasErrorCode {
				kvs = append(kvs, "error_code", errorCode, "error_type", errorType)
			}
			if hasStatusCode {
				kvs = append(kvs, "status", statusCode)
			}
			if err != nil {
				kvs = append(kvs, "err", err.Error())
			}
			pushlog(ctx, kvs, err)
			return response, err
		}
	}
}

func addStatsKVs(kvs []interface{}, ri *rpcInfo) []interface{} {
	cwp, ok := ri.Conn().(*connpool.ConnWithPkgSize)
	if ok {
		kvs = append(kvs,
			"written_size", atomic.LoadInt32(&cwp.Written),
			"read_size", atomic.LoadInt32(&cwp.Readn),
		)
	}
	return kvs
}

// {Data} {Time} {Location} {HostIP} {PSM} {LogID} {Cluster} {Stage} {KV1} {KV2} ...
func formatLog(basic []string, kvs []interface{}) string {
	b := make([]byte, 0, 4096)
	b = append(b, time.Now().Format(localLayout)...)
	b = append(b, ' ')
	b = append(b, localLogVersion...)
	b = append(b, ' ')
	for _, s := range basic {
		b = append(b, s...)
		b = append(b, ' ')
	}

	for i := 0; i < len(kvs); i += 2 {
		b = append(b, kvs[i].(string)...)
		b = append(b, '=')
		switch kvs[i+1].(type) {
		case string:
			b = append(b, kvs[i+1].(string)...)
		case int:
			b = strconv.AppendInt(b, int64(kvs[i+1].(int)), 10)
		case int64:
			b = strconv.AppendInt(b, kvs[i+1].(int64), 10)
		}
		b = append(b, ' ')
	}
	return string(b)
}
