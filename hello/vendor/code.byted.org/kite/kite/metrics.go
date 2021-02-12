package kite

/*
Metrics 定义描述 https://wiki.bytedance.com/pages/viewpage.action?pageId=51348664
*/

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"code.byted.org/gopkg/logs"
	"code.byted.org/gopkg/metrics"
	"code.byted.org/gopkg/stats"
	"code.byted.org/kite/endpoint"
	"code.byted.org/kite/kitutil/kiterrno"
)

const (
	namespacePrefix string = "toutiao"
)

var (
	metricsClient MetricsEmiter
)

func init() {
	// IgnoreCheck is true.
	metricsClient = metrics.NewDefaultMetricsClientV2(namespacePrefix, true)
}

// GoStatMetrics emit GC, Stask, Heap info to TSDB.
func GoStatMetrics() error {
	return stats.DoReport(ServiceName)
}

// MetricsEmiter .
type MetricsEmiter interface {
	EmitCounter(name string, value interface{}, tagkv ...metrics.T) error
	EmitTimer(name string, value interface{}, tagkv ...metrics.T) error
	EmitStore(name string, value interface{}, tagkv ...metrics.T) error
}

type MetricsEmiterEnhance interface {
	MetricsEmiter
	EmitRateCounter(name string, value interface{}, tagkv ...metrics.T) error
	EmitMeter(name string, value interface{}, tagkv ...metrics.T) error
}

type adaptCounterToMeter struct{ MetricsEmiterEnhance }

func (a *adaptCounterToMeter) EmitCounter(name string, value interface{}, tags ...metrics.T) error {
	return a.EmitMeter(name, value, tags...)
}

func EnableMetricsCounterAsMeter() {
	if mh, ok := metricsClient.(MetricsEmiterEnhance); ok {
		metricsClient = &adaptCounterToMeter{mh}
	} else {
		logs.Warnf("The current metrics client does not support 'meter'")
	}
}

// EmptyEmiter .
type EmptyEmiter struct{}

// EmitCounter .
func (ee *EmptyEmiter) EmitCounter(name string, value interface{}, tagkv ...metrics.T) error {
	return nil
}

// EmitTimer .
func (ee *EmptyEmiter) EmitTimer(name string, value interface{}, tagkv ...metrics.T) error {
	return nil
}

// EmitStore .
func (ee *EmptyEmiter) EmitStore(name string, value interface{}, tagkv ...metrics.T) error {
	return nil

}

// server site metrics format
const (
	successThroughputFmt   string = "service.thrift.%s.%s.calledby.success.throughput"
	errorThroughputFmt     string = "service.thrift.%s.%s.calledby.error.throughput"
	successLatencyFmt      string = "service.thrift.%s.%s.calledby.success.latency.us"
	errorLatencyFmt        string = "service.thrift.%s.%s.calledby.error.latency.us"
	accessTotalFmt         string = "service.request.%s.total"
	totalSuccessLatencyFmt string = "service.thrift.%s.calledby.success.latency.us"
	totalErrorLatencyFmt   string = "service.thrift.%s.calledby.error.latency.us"

	statusSuccess string = "success"
	statusFailed  string = "failed"
)

func getStandardMetricsTags(rpc *RPCInfo, useMapTags bool) (map[string]string, []metrics.T) {
	upstreamService := empty2Default(rpc.UpstreamService, "-")
	upstreamCluster := empty2Default(rpc.UpstreamCluster, "-")

	if useMapTags {
		tags := map[string]string{
			"mesh":         ifelse(ServiceMeshMode, "1", "0"),
			"from":         upstreamService,
			"from_cluster": upstreamCluster,
			"to_cluster":   rpc.Cluster,
			"stress_tag":   empty2Default(rpc.StressTag, "-"),
		}
		return tags, nil
	} else {
		tags := []metrics.T{
			{"mesh", ifelse(ServiceMeshMode, "1", "0")},
			{"from", upstreamService},
			{"from_cluster", upstreamCluster},
			{"to_cluster", rpc.Cluster},
			{"stress_tag", empty2Default(rpc.StressTag, "-")},
		}
		return nil, tags
	}
}

type metricsControlV2 func(rpcCtx *kiterrno.RPCContext, tags []metrics.T) (isError bool, newTags []metrics.T)

// AccessMetricsMW emit access metrics
func AccessMetricsMW(next endpoint.EndPoint) endpoint.EndPoint {
	var useMapTags bool = false
	var decisionFuncV2 metricsControlV2

	decisionFunc := kiteMetricsControl
	cmc := fmt.Sprint(kiterrno.MetricsControl(CompatibleMetrics))
	umc := fmt.Sprint(kiterrno.MetricsControl(UncompatibleMetrics))
	switch fmt.Sprint(decisionFunc) {
	case cmc:
		decisionFuncV2 = CompatibleMetricsV2
	case umc:
		decisionFuncV2 = UncompatibleMetricsV2
	default:
		// Customized
		useMapTags = true
	}

	return func(ctx context.Context, request interface{}) (interface{}, error) {
		if ctx == nil {
			return next(ctx, request)
		}
		begin := time.Now()
		response, err := next(ctx, request)
		took := time.Since(begin).Microseconds()

		var isError bool
		r := GetRPCInfo(ctx)

		var mtags []metrics.T
		if useMapTags {
			tags, _ := getStandardMetricsTags(r, useMapTags)
			isError, tags = decisionFunc(&kiterrno.RPCContext{ctx, request, response, err}, tags)

			// Throughput and latency metrics
			tname := ifelse(isError, errorThroughputFmt, successThroughputFmt)
			lname := ifelse(isError, errorLatencyFmt, successLatencyFmt)

			tname = fmt.Sprintf(tname, r.Service, r.Method)
			lname = fmt.Sprintf(lname, r.Service, r.Method)

			mtags = metrics.Map2Tags(tags)
			metricsClient.EmitCounter(tname, 1, mtags...)
			metricsClient.EmitTimer(lname, took, mtags...)

			accessTotal := fmt.Sprintf(accessTotalFmt, r.Service)
			metricsClient.EmitCounter(accessTotal, 1, mtags...)
		} else {
			_, tags := getStandardMetricsTags(r, useMapTags)
			isError, tags = decisionFuncV2(&kiterrno.RPCContext{ctx, request, response, err}, tags)

			// Throughput and latency metrics
			tname := ifelse(isError, errorThroughputFmt, successThroughputFmt)
			lname := ifelse(isError, errorLatencyFmt, successLatencyFmt)

			tname = fmt.Sprintf(tname, r.Service, r.Method)
			lname = fmt.Sprintf(lname, r.Service, r.Method)

			metricsClient.EmitCounter(tname, 1, tags...)
			metricsClient.EmitTimer(lname, took, tags...)

			accessTotal := fmt.Sprintf(accessTotalFmt, r.Service)
			metricsClient.EmitCounter(accessTotal, 1, tags...)

			mtags = tags
		}

		if stats, ok := getRPCStats(ctx); ok {
			lname := ifelse(isError, totalErrorLatencyFmt, totalSuccessLatencyFmt)
			lname = fmt.Sprintf(lname, r.Service)
			mtags = append(mtags, metrics.T{Name: "method", Value: r.Method})

			stats.AddCallback(func(ss *rpcStats) {
				took := ss.TotalCostUS()
				metricsClient.EmitTimer(lname, took, mtags...)
			})
		}

		return response, err
	}
}

func GetResultCodes(err error, resp interface{}) (hasErrorCode, hasStatusCode bool, statusCode, errorCode, errorType int) {
	if kerr, ok := err.(kiterrno.KiteError); ok {
		errorCode, errorType = kerr.Errno(), kerr.Category()
		hasErrorCode = true
	}

	statusCode, hasStatusCode = kiterrno.GetErrCode(resp)
	return
}

func CompatibleMetrics(rpcCtx *kiterrno.RPCContext, tags map[string]string) (bool, map[string]string) {
	hasErrorCode, _, statusCode, errorCode, _ := GetResultCodes(rpcCtx.Error, rpcCtx.Response)

	code, isError := 0, rpcCtx.Error != nil

	if !isError {
		code = statusCode
	} else {
		if hasErrorCode {
			code = errorCode
		}
	}

	tags["status_code"] = strconv.Itoa(code)
	return isError, tags
}

func UncompatibleMetrics(rpcCtx *kiterrno.RPCContext, tags map[string]string) (bool, map[string]string) {
	hasErrorCode, hasStatusCode, statusCode, errorCode, errorType := GetResultCodes(rpcCtx.Error, rpcCtx.Response)

	isError := rpcCtx.Error != nil

	if hasStatusCode {
		tags["status_code"] = strconv.Itoa(statusCode)
	}
	if hasErrorCode {
		tags["error_code"] = strconv.Itoa(errorCode)
		tags["error_type"] = strconv.Itoa(errorType)
	}
	return isError, tags
}

func CompatibleMetricsV2(rpcCtx *kiterrno.RPCContext, tags []metrics.T) (bool, []metrics.T) {
	hasErrorCode, _, statusCode, errorCode, _ := GetResultCodes(rpcCtx.Error, rpcCtx.Response)

	code, isError := 0, rpcCtx.Error != nil

	if !isError {
		code = statusCode
	} else {
		if hasErrorCode {
			code = errorCode
		}
	}

	tags = append(tags, metrics.T{"status_code", strconv.Itoa(code)})
	return isError, tags
}

func UncompatibleMetricsV2(rpcCtx *kiterrno.RPCContext, tags []metrics.T) (bool, []metrics.T) {
	hasErrorCode, hasStatusCode, statusCode, errorCode, errorType := GetResultCodes(rpcCtx.Error, rpcCtx.Response)

	isError := rpcCtx.Error != nil

	if hasStatusCode {
		tags = append(tags, metrics.T{"status_code", strconv.Itoa(statusCode)})
	}
	if hasErrorCode {
		tags = append(tags, metrics.T{"error_code", strconv.Itoa(errorCode)})
		tags = append(tags, metrics.T{"error_type", strconv.Itoa(errorType)})
	}
	return isError, tags
}
