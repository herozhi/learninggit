package kitutil

import (
	"code.byted.org/kite/kitc/discovery"
	"context"
	"time"
)

const (
	LOGIDKEY         = "K_LOGID"         // 唯一的Request ID
	CALLERKEY        = "K_CALLER"        // 上游服务的名字
	CALLERCLUSTERKEY = "K_CALLERCLUSTER" // 上游服务的集群名字
	ENVKEY           = "K_ENV"           // 上游服务带过来的环境参数
	CLIENTKEY        = "K_CLIENT"        // 客户端的标识，目前保留
	ADDRKEY          = "K_ADDR"          // 上游服务的IP地址
	LOCALIPKEY       = "K_LOCALIP"       // 本服务的IP 地址
	METHODKEY        = "K_METHOD"        // 本服务当前所处的接口名字（也就是Method名字）
	SNAMEKEY         = "K_SNAME"         // 本服务的名字
	CLUSTERKEY       = "K_CLUSTER"       // 本服务集群的名字
	STRESSTAG        = "K_STRESS"        // 压测流量
	OPENTRACING      = "K_TRACE"         // 传递 trace 的状态 (eg. TraceID:ParentSpanID:flag)
	MAXFRAMESIZE     = "K_MAXFREAMESIZE" // 动态设置frame最大包大小
	INSTANCES        = "K_INSTANCES"     // 指定下游服务地址

	NSNAMEKEY   = "K_NSNAME"   // 下游服务名字
	NCLUSTERKEY = "K_NCLUSTER" // 下游服务集群的名字
	NMNAMEKEY   = "K_NMNAME"   // 下游服务的接口名字

	// 兼容性字段
	RPC_TIMEOUT_KEY = "K_RPC_TIMEOUT"

	// user-defined extra fields
	// 为了避免A->B无意识的透传, 处理时会区分上下游
	UPSTREAM_USER_EXTRA = "K_UPSTREAM_USER_EXTRA"

	// 用户自定义路由的ID
	DDP_ROUTING_TAG = "K_DDP_ROUTING_TAG"
)

func GetCtxUserExtra(ctx context.Context, key string) (string, bool) {
	if ctx == nil {
		return "", false
	}
	m, ok := ctx.Value(UPSTREAM_USER_EXTRA).(map[string]string)
	if !ok {
		return "", false
	}
	v, ok := m[key]
	return v, ok
}

func GetCtxUserExtraAll(ctx context.Context) (map[string]string, bool) {
	if ctx == nil {
		return nil, false
	}
	m, ok := ctx.Value(UPSTREAM_USER_EXTRA).(map[string]string)
	return m, ok
}

func NewCtxWithUpstreamUserExtra(ctx context.Context, m map[string]string) context.Context {
	return context.WithValue(ctx, UPSTREAM_USER_EXTRA, m)
}

func AddCtxUserExtra(ctx context.Context, key, val string) context.Context {
	return ctxAddKV(ctx, key, val)
}

func GetCtxDownstreamUserExtra(ctx context.Context) (map[string]string, bool) {
	m := ctxGetAll(ctx)
	if m == nil {
		return nil, false
	}
	return m, true
}

// getStrCtx read the value of key in ctx, return it in string type.
func getStrCtx(ctx context.Context, key string) (string, bool) {
	if ctx == nil {
		return "", false
	}
	v := ctx.Value(key)
	switch v := v.(type) {
	case string:
		return v, true
	case *string:
		return *v, true
	}
	return "", false
}

func GetCtxMaxFrameSize(ctx context.Context) (int, bool) {
	if ctx == nil {
		return 0, false
	}
	v, ok := ctx.Value(MAXFRAMESIZE).(int)
	if !ok {
		return 0, false
	}
	return v, true
}

func NewCtxWithMaxFrameSize(ctx context.Context, frameSize int) context.Context {
	return context.WithValue(ctx, MAXFRAMESIZE, frameSize)
}

// GetCtxLogID return logid store in ctx.
func GetCtxLogID(ctx context.Context) (string, bool) {
	return getStrCtx(ctx, LOGIDKEY)
}

// NewCtxWithLogID .
func NewCtxWithLogID(ctx context.Context, logID string) context.Context {
	return context.WithValue(ctx, LOGIDKEY, logID)
}

// GetCtxCaller .
func GetCtxCaller(ctx context.Context) (string, bool) {
	return getStrCtx(ctx, CALLERKEY)
}

// NewCtxWithCaller .
func NewCtxWithCaller(ctx context.Context, caller string) context.Context {
	return context.WithValue(ctx, CALLERKEY, caller)
}

// GetCtxEnv .
func GetCtxEnv(ctx context.Context) (string, bool) {
	return getStrCtx(ctx, ENVKEY)
}

// NewCtxWithEnv .
func NewCtxWithEnv(ctx context.Context, env string) context.Context {
	return context.WithValue(ctx, ENVKEY, env)
}

// GetCtxCluster .
func GetCtxCluster(ctx context.Context) (string, bool) {
	return getStrCtx(ctx, CLUSTERKEY)
}

// NewCtxWithCluster .
func NewCtxWithCluster(ctx context.Context, cluster string) context.Context {
	return context.WithValue(ctx, CLUSTERKEY, cluster)
}

// GetCtxCallerCluster .
func GetCtxCallerCluster(ctx context.Context) (string, bool) {
	return getStrCtx(ctx, CALLERCLUSTERKEY)
}

// NewCtxWithCallerCluster .
func NewCtxWithCallerCluster(ctx context.Context, cluster string) context.Context {
	return context.WithValue(ctx, CALLERCLUSTERKEY, cluster)
}

// GetCtxClient .
func GetCtxClient(ctx context.Context) (string, bool) {
	return getStrCtx(ctx, CLIENTKEY)
}

// NewCtxWithClient .
func NewCtxWithClient(ctx context.Context, client string) context.Context {
	return context.WithValue(ctx, CLIENTKEY, client)
}

// GetCtxAddr .
func GetCtxAddr(ctx context.Context) (string, bool) {
	return getStrCtx(ctx, ADDRKEY)
}

// NewCtxWithAddr .
func NewCtxWithAddr(ctx context.Context, addr string) context.Context {
	return context.WithValue(ctx, ADDRKEY, addr)
}

// GetCtxLocalIP .
func GetCtxLocalIP(ctx context.Context) (string, bool) {
	return getStrCtx(ctx, LOCALIPKEY)
}

// NewCtxWithLocalIP .
func NewCtxWithLocalIP(ctx context.Context, localIP string) context.Context {
	return context.WithValue(ctx, LOCALIPKEY, localIP)
}

// GetCtxMethod .
func GetCtxMethod(ctx context.Context) (string, bool) {
	return getStrCtx(ctx, METHODKEY)
}

// NewCtxWithMethod .
func NewCtxWithMethod(ctx context.Context, method string) context.Context {
	return context.WithValue(ctx, METHODKEY, method)
}

// GetCtxServiceName .
func GetCtxServiceName(ctx context.Context) (string, bool) {
	return getStrCtx(ctx, SNAMEKEY)
}

// NewCtxWithServiceName .
func NewCtxWithServiceName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, SNAMEKEY, name)
}

// GetCtxTargetServiceName .
func GetCtxTargetServiceName(ctx context.Context) (string, bool) {
	return getStrCtx(ctx, NSNAMEKEY)
}

// NewCtxWithTargetServiceName .
func NewCtxWithTargetServiceName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, NSNAMEKEY, name)
}

// GetCtxTargetClusterName .
func GetCtxTargetClusterName(ctx context.Context) (string, bool) {
	return getStrCtx(ctx, NCLUSTERKEY)
}

// NewCtxWithTargetCluster .
func NewCtxWithTargetClusterName(ctx context.Context, cluster string) context.Context {
	return context.WithValue(ctx, NCLUSTERKEY, cluster)
}

// GetCtxTargetMethod .
func GetCtxTargetMethod(ctx context.Context) (string, bool) {
	return getStrCtx(ctx, NMNAMEKEY)
}

// NewCtxWithTargetMethod .
func NewCtxWithTargetMethod(ctx context.Context, method string) context.Context {
	return context.WithValue(ctx, NMNAMEKEY, method)
}

// GetCtxWithDefault return val from ctx, if the value is not exist or is "", return default val.
func GetCtxWithDefault(f func(ctx context.Context) (string, bool), ctx context.Context, val string) string {
	v, ok := f(ctx)
	if !ok || v == "" {
		return val
	}
	return v
}

// NewCtxWithTargetConn .
func NewCtxWithRPCTimeout(ctx context.Context, timeout time.Duration) context.Context {
	return context.WithValue(ctx, RPC_TIMEOUT_KEY, timeout)
}

// GetCtxRPCTimeout .
func GetCtxRPCTimeout(ctx context.Context) (time.Duration, bool) {
	if ctx == nil {
		return 0, false
	}
	v, ok := ctx.Value(RPC_TIMEOUT_KEY).(time.Duration)
	if !ok {
		return 0, false
	}
	return v, true
}

// NewCtxWithStressTag .
func NewCtxWithStressTag(ctx context.Context, stressTag string) context.Context {
	return context.WithValue(ctx, STRESSTAG, stressTag)
}

// GetCtxStressTag .
func GetCtxStressTag(ctx context.Context) (string, bool) {
	return getStrCtx(ctx, STRESSTAG)
}

// NewCtxWithTraceTag
func NewCtxWithTraceTag(ctx context.Context, traceTag string) context.Context {
	return context.WithValue(ctx, OPENTRACING, traceTag)
}

// GetCtxTraceTag .
func GetCtxTraceTag(ctx context.Context) (string, bool) {
	return getStrCtx(ctx, OPENTRACING)
}

func NewCtxWithDDPRoutingTag(ctx context.Context, tag string) context.Context {
	return context.WithValue(ctx, DDP_ROUTING_TAG, tag)
}

func GetCtxDDPRoutingTag(ctx context.Context) (string, bool) {
	return getStrCtx(ctx, DDP_ROUTING_TAG)
}

// NewCtxWithRPCInstances .
func NewCtxWithRPCInstances(ctx context.Context, instances []*discovery.Instance) context.Context {
	return context.WithValue(ctx, INSTANCES, instances)
}

// GetCtxRPCInstances .
func GetCtxRPCInstances(ctx context.Context) ([]*discovery.Instance, bool) {
	if ctx == nil {
		return nil, false
	}
	v, ok := ctx.Value(INSTANCES).([]*discovery.Instance)
	if !ok {
		return nil, false
	}
	return v, true
}
