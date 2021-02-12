package kitc

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"code.byted.org/gopkg/logs"
	"code.byted.org/gopkg/metrics"
	"code.byted.org/kite/endpoint"
	"code.byted.org/kite/kitutil/kiterrno"
)

type MetricsEmiter interface {
	EmitCounter(name string, value interface{}, tags ...metrics.T) error
	EmitTimer(name string, value interface{}, tags ...metrics.T) error
	EmitStore(name string, value interface{}, tags ...metrics.T) error
}

type MetricsEmiterEnhance interface {
	MetricsEmiter
	EmitRateCounter(name string, value interface{}, tags ...metrics.T) error
	EmitMeter(name string, value interface{}, tags ...metrics.T) error
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

var (
	metricsClient MetricsEmiter
)

const (
	namespacePrefix string = "toutiao"
)

func init() {
	metricsClient = metrics.NewDefaultMetricsClientV2(namespacePrefix, true)
}

// client site metrics format
const (
	successRPCThroughputFmt string = "service.thrift.%s.call.success.throughput"
	errorRPCThroughputFmt   string = "service.thrift.%s.call.error.throughput"
	successRPCLatencyFmt    string = "service.thrift.%s.call.success.latency.us"
	errorRPCLatencyFmt      string = "service.thrift.%s.call.error.latency.us"

	stabilityFmt string = "service.stability.%s.throughput"
)

func getStandardMetricsTags(rpc *rpcInfo, useMapTags bool) (map[string]string, []metrics.T) {
	if useMapTags {
		tags := map[string]string{
			"mesh":         ifelse(ServiceMeshMode, "1", "0"),
			"from":         rpc.From,
			"from_cluster": rpc.FromCluster,
			"to":           rpc.To,
			"method":       rpc.Method,
			"to_idc":       rpc.TargetIDC(),
			"to_cluster":   ifelse(rpc.ToCluster != "", rpc.ToCluster, "default"),
			"stress_tag":   ifelse(rpc.StressTag != "", rpc.StressTag, "-"),
		}
		return tags, nil
	} else {
		tags := []metrics.T{
			{"mesh", ifelse(ServiceMeshMode, "1", "0")},
			{"from", rpc.From},
			{"from_cluster", rpc.FromCluster},
			{"to", rpc.To},
			{"method", rpc.Method},
			{"to_idc", rpc.TargetIDC()},
			{"to_cluster", ifelse(rpc.ToCluster != "", rpc.ToCluster, "default")},
			{"stress_tag", ifelse(rpc.StressTag != "", rpc.StressTag, "-")},
		}
		return nil, tags
	}
}

type metricsControlV2 func(rpcCtx *kiterrno.RPCContext, tags []metrics.T) (isError bool, newTags []metrics.T)

// RPCMetricsMW .
func RPCMetricsMW(mwCtx context.Context) endpoint.Middleware {
	var useMapTags bool
	var decisionFuncV2 metricsControlV2

	decisionFunc := mwCtx.Value(KITC_OPTIONS_KEY).(*Options).MetricsControl
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

	addToHostTag := mwCtx.Value(KITC_OPTIONS_KEY).(*Options).AddToHostTagToMetrics

	return func(next endpoint.EndPoint) endpoint.EndPoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			begin := time.Now()
			response, err := next(ctx, request)
			took := time.Since(begin).Nanoseconds() / 1000 //us

			if kerr, ok := err.(kiterrno.KiteError); ok && kerr.Category() == kiterrno.KITE {
				switch kerr.Errno() {
				case kiterrno.NotAllowedByACLCode,
					kiterrno.ForbiddenByDegradationCode:
					// Do not output metrics on these errors
					return response, err
				}
			}

			var isError bool
			rpcInfo := GetRPCInfo(ctx)

			if useMapTags {
				tags, _ := getStandardMetricsTags(rpcInfo, useMapTags)
				if addToHostTag {
					remoteIP := rpcInfo.GetRemoteIP(ctx, response)
					tags["to_host"] = remoteIP
				}
				if ServiceMeshMode {
					if toCluster := rpcInfo.GetToCluster(ctx, response); toCluster != "" {
						tags["to_cluster"] = toCluster
					}
				}

				isError, tags = decisionFunc(&kiterrno.RPCContext{ctx, request, response, err}, tags)

				// Throughput and latency metrics
				tname := ifelse(isError, errorRPCThroughputFmt, successRPCThroughputFmt)
				lname := ifelse(isError, errorRPCLatencyFmt, successRPCLatencyFmt)

				tname = fmt.Sprintf(tname, rpcInfo.From)
				lname = fmt.Sprintf(lname, rpcInfo.From)

				metricsClient.EmitCounter(tname, 1, metrics.Map2Tags(tags)...)
				metricsClient.EmitTimer(lname, took, metrics.Map2Tags(tags)...)

				// Stability metrics
				stabilityMetrics := fmt.Sprintf(stabilityFmt, rpcInfo.To)
				metricsClient.EmitCounter(stabilityMetrics, 1, metrics.Map2Tags(tags)...)
			} else {
				_, tags := getStandardMetricsTags(rpcInfo, useMapTags)
				if addToHostTag {
					remoteIP := rpcInfo.GetRemoteIP(ctx, response)
					tags = append(tags, metrics.T{"to_host", remoteIP})
				}

				if ServiceMeshMode {
					if toCluster := rpcInfo.GetToCluster(ctx, response); toCluster != "" {
						for i := range tags {
							if tags[i].Name == "to_cluster" {
								tags[i].Value = toCluster
								break
							}
						}
					}
				}

				isError, tags = decisionFuncV2(&kiterrno.RPCContext{ctx, request, response, err}, tags)

				// Throughput and latency metrics
				tname := ifelse(isError, errorRPCThroughputFmt, successRPCThroughputFmt)
				lname := ifelse(isError, errorRPCLatencyFmt, successRPCLatencyFmt)

				tname = fmt.Sprintf(tname, rpcInfo.From)
				lname = fmt.Sprintf(lname, rpcInfo.From)

				metricsClient.EmitCounter(tname, 1, tags...)
				metricsClient.EmitTimer(lname, took, tags...)

				// Stability metrics
				stabilityMetrics := fmt.Sprintf(stabilityFmt, rpcInfo.To)
				metricsClient.EmitCounter(stabilityMetrics, 1, tags...)
			}

			return response, err
		}
	}
}

func CompatibleMetrics(rpcCtx *kiterrno.RPCContext, tags map[string]string) (bool, map[string]string) {
	isError, code, hasCode := rpcCtx.Error != nil, 0, false

	if kerr, ok := rpcCtx.Error.(kiterrno.KiteError); ok {
		switch kerr.Category() {
		case kiterrno.THRIFT, kiterrno.KITE, kiterrno.MESH:
			code = kerr.Errno()
			hasCode = true
		}
	}

	if !isError || !hasCode {
		if errCode, ok := kiterrno.GetErrCode(rpcCtx.Response); ok {
			code = errCode
		}
	}

	if code < 0 {
		tags["label"] = "business_err"
	} else if kiterrno.IsKiteErrCode(code) || kiterrno.IsMeshErrorCode(code) {
		tags["label"] = "net_err"
	} else {
		tags["label"] = "success"
	}

	if isError || tags["label"] != "success" {
		tags["err_code"] = strconv.Itoa(code)
	}
	return isError, tags
}

func GetResultCodes(err error, resp interface{}) (hasErrorCode, hasStatusCode bool, statusCode, errorCode, errorType int) {
	if kerr, ok := err.(kiterrno.KiteError); ok {
		errorCode, errorType = kerr.Errno(), kerr.Category()
		hasErrorCode = true
	}

	statusCode, hasStatusCode = kiterrno.GetErrCode(resp)
	return
}

func UncompatibleMetrics(rpcCtx *kiterrno.RPCContext, tags map[string]string) (bool, map[string]string) {
	hasErrorCode, hasStatusCode, statusCode, errorCode, errorType := GetResultCodes(rpcCtx.Error, rpcCtx.Response)

	isError := rpcCtx.Error != nil

	if hasStatusCode {
		tags["status_code"] = strconv.Itoa(statusCode)
	}
	if hasErrorCode {
		tags["err_type"] = strconv.Itoa(errorType)
		tags["err_code"] = strconv.Itoa(errorCode)
	}
	tags["label"] = ifelse(isError, "failure", "success")
	return isError, tags
}

func CompatibleMetricsV2(rpcCtx *kiterrno.RPCContext, tags []metrics.T) (bool, []metrics.T) {
	isError, code, hasCode := rpcCtx.Error != nil, 0, false

	if kerr, ok := rpcCtx.Error.(kiterrno.KiteError); ok {
		switch kerr.Category() {
		case kiterrno.THRIFT, kiterrno.KITE, kiterrno.MESH:
			code = kerr.Errno()
			hasCode = true
		}
	}

	if !isError || !hasCode {
		if errCode, ok := kiterrno.GetErrCode(rpcCtx.Response); ok {
			code = errCode
		}
	}

	var label string
	if code < 0 {
		label = "business_err"
	} else if kiterrno.IsKiteErrCode(code) || kiterrno.IsMeshErrorCode(code) {
		label = "net_err"
	} else {
		label = "success"
	}
	tags = append(tags, metrics.T{"label", label})

	if isError || label != "success" {
		tags = append(tags, metrics.T{"err_code", strconv.Itoa(code)})
	}
	return isError, tags
}

func UncompatibleMetricsV2(rpcCtx *kiterrno.RPCContext, tags []metrics.T) (bool, []metrics.T) {
	hasErrorCode, hasStatusCode, statusCode, errorCode, errorType := GetResultCodes(rpcCtx.Error, rpcCtx.Response)

	isError := rpcCtx.Error != nil

	if hasStatusCode {
		tags = append(tags, metrics.T{"status_code", strconv.Itoa(statusCode)})
	}
	if hasErrorCode {
		tags = append(tags, metrics.T{"err_type", strconv.Itoa(errorType)})
		tags = append(tags, metrics.T{"err_code", strconv.Itoa(errorCode)})
	}
	tags = append(tags, metrics.T{"label", ifelse(isError, "failure", "success")})
	return isError, tags
}
