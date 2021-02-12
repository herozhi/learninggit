package kiterrno

import (
	"context"

	"code.byted.org/kite/endpoint"
)

// GetStatusCode extracts the status code from response
func GetStatusCode(resp interface{}) (int, bool) {
	kitResp, ok := resp.(endpoint.KiteResponse)
	if !ok { // if resp is invalid, return directly
		return 0, false
	}

	baseResp := kitResp.GetBaseResp()
	if baseResp == nil {
		return 0, false
	}

	code := baseResp.GetStatusCode()
	return int(code), true
}

var GetErrCode = GetStatusCode // For compatibility only

// RPCContext is a container that holds all information during one RPC process
type RPCContext struct {
	Ctx      context.Context
	Request  interface{}
	Response interface{}
	Error    error
}

// MetricsControl is a handler/filter that user could provide to control the process of
// generating rpc or access metrics.
// MetricsControl will be invoked in kitc's `RPCMetricsMW` or kite's `AccessMetricsMW` to determine
// whether the current response and error should be regarded as "error" and to determine what
// tags should be used in metrics.
type MetricsControl func(
	rpcCtx *RPCContext,
	tags map[string]string,
) (isError bool, newTags map[string]string)
