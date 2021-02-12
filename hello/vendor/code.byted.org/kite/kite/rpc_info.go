package kite

import "context"

type ttheaderIntKeyType = uint16

// just list some keys about rpcinfo
// ref: https://bytedance.feishu.cn/docs/YkTSMOwaEI10ispwcNK76c#flhAjt
const (
	ikLogID       ttheaderIntKeyType = 2
	ikFromService                    = 3
	ikFromCluster                    = 4
	ikFromIDC                        = 5
	ikEnv                            = 10
	ikStressTag                      = 21
)

// RPCMeta .
type RPCMeta struct {
	Service         string
	Cluster         string
	UpstreamService string
	UpstreamCluster string
	Method          string
}

func (r RPCMeta) String() string {
	sum := len(r.UpstreamService) + len(r.UpstreamCluster) +
		len(r.Service) + len(r.Cluster) + len(r.Method) + 4
	buf := make([]byte, 0, sum)
	buf = append(buf, r.UpstreamService...)
	buf = append(buf, '/')
	buf = append(buf, r.UpstreamCluster...)
	buf = append(buf, '/')
	buf = append(buf, r.Service...)
	buf = append(buf, '/')
	buf = append(buf, r.Cluster...)
	buf = append(buf, '/')
	buf = append(buf, r.Method...)
	return string(buf)
}

// RPCConfig .
type RPCConfig struct {
	ACLAllow        bool
	StressBotSwitch bool
}

// RPCInfo .
type RPCInfo struct {
	RPCMeta
	RPCConfig

	UpstreamIDC string // now only from mesh egress ttheader
	// extra info
	LogID           string
	Client          string
	Env             string
	LocalIP         string
	RemoteIP        string // upstream IP
	StressTag       string
	TraceTag        string // opentracing context passed by request-extra
	RingHashKey     string
	DDPTag          string
	httpContentType string // use for mesh protocol convert
}

type rpcInfoCtxKey struct{}

// GetRPCInfo .
func GetRPCInfo(ctx context.Context) *RPCInfo {
	return ctx.Value(rpcInfoCtxKey{}).(*RPCInfo)
}

func newCtxWithRPCInfo(ctx context.Context, rpcInfo *RPCInfo) context.Context {
	return context.WithValue(ctx, rpcInfoCtxKey{}, rpcInfo)
}

// SetHttpContentType use for mesh protocol convert
func SetHttpContentType(ctx context.Context, httpContentType string) {
	r := GetRPCInfo(ctx)
	r.httpContentType = httpContentType
}
