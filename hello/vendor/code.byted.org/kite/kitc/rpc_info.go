package kitc

import (
	"context"
	"net"
	"sync"
	"time"

	"code.byted.org/gopkg/metainfo"
	"code.byted.org/kite/endpoint"
	"code.byted.org/kite/kitc/discovery"
)

type RingHashKeyType string

// IDCConfig .
type IDCConfig struct {
	IDC     string
	Percent int // total is 100
}

// CBConfig .
type CBConfig struct {
	IsOpen    bool
	ErrRate   float64
	MinSample int64
}

// InstanceCBConfig 根据传入的参数进行判断，分别采用以下三种策略：
// 1. 当样本数 >= MinSample 且 错误率 >= ErrRate
// 2. 当样本数 >= DurationSamples 且 连续出错时长 >= Duration
// 3. 当连续错误数 >= ConseErrors
// 以上三种策略成立任何一种就打开熔断器。
// 如果 Duration 为 0 表示不启用策略 2
// 如果 ConseErrors 为 0 表示不启用策略 3
type InstanceCBConfig struct {
	IsOpen    bool
	ErrRate   float64
	MinSample int64

	Duration        time.Duration
	DurationSamples int64

	ConseErrors int64
}

func (cb CBConfig) Valid() bool {
	return cb.ErrRate > 0 && cb.MinSample > 0
}

func (cb InstanceCBConfig) Valid() bool {
	return cb.ErrRate > 0 && cb.MinSample > 0
}

// RPCConfig .
type RPCConfig struct {
	RPCTimeout      int // ms
	ConnectTimeout  int // ms
	ReadTimeout     int // ms
	WriteTimeout    int // ms
	IDCConfig       []IDCConfig
	ServiceCB       CBConfig
	ACLAllow        bool
	DegraPercent    int
	StressBotSwitch bool
}

// RPCMeta .
type RPCMeta struct {
	From        string
	FromCluster string
	FromMethod  string
	To          string
	ToCluster   string
	Method      string
}

func (r RPCMeta) ConfigKey() string {
	toCluster := ifelse(r.ToCluster != "", r.ToCluster, "default")
	sum := len(r.From) + len(r.FromCluster) + len(r.To) + len(toCluster) + len(r.Method) + 4
	buf := make([]byte, 0, sum)
	buf = append(buf, r.From...)
	buf = append(buf, '/')
	buf = append(buf, r.FromCluster...)
	buf = append(buf, '/')
	buf = append(buf, r.To...)
	buf = append(buf, '/')
	buf = append(buf, toCluster...)
	buf = append(buf, '/')
	buf = append(buf, r.Method...)
	return string(buf)
}

func (r RPCMeta) String() string {
	sum := len(r.From) + len(r.FromCluster) + len(r.FromMethod) + len(r.To) + len(r.ToCluster) + len(r.Method) + 5
	buf := make([]byte, 0, sum)
	buf = append(buf, r.From...)
	buf = append(buf, '/')
	buf = append(buf, r.FromCluster...)
	buf = append(buf, '/')
	buf = append(buf, r.FromMethod...)
	buf = append(buf, '/')
	buf = append(buf, r.To...)
	buf = append(buf, '/')
	buf = append(buf, r.ToCluster...)
	buf = append(buf, '/')
	buf = append(buf, r.Method...)
	return string(buf)
}

type rpcInfo struct {
	sync.RWMutex
	RPCMeta
	RPCConfig

	// extra info, modified by middlewares
	LocalIP   string
	Env       string
	LogID     string
	Client    string
	ConnCost  int32
	StressTag string
	TraceTag  string

	// ringhash key(FIXME)
	RingHashKey string

	targetIDC      string
	targetInstance *discovery.Instance
	instances      []*discovery.Instance
	conn           net.Conn
}

func (r *rpcInfo) Conn() net.Conn {
	r.RLock()
	defer r.RUnlock()
	return r.conn
}

func (r *rpcInfo) SetConn(conn net.Conn) {
	r.Lock()
	defer r.Unlock()
	r.conn = conn
}

func (r *rpcInfo) Instances() []*discovery.Instance {
	r.RLock()
	defer r.RUnlock()
	return r.instances
}

func (r *rpcInfo) SetInstances(inss []*discovery.Instance) {
	r.Lock()
	defer r.Unlock()
	r.instances = inss
}

func (r *rpcInfo) TargetInstance() *discovery.Instance {
	r.RLock()
	defer r.RUnlock()
	return r.targetInstance
}

func (r *rpcInfo) SetTargetInstance(i *discovery.Instance) {
	r.Lock()
	defer r.Unlock()
	r.targetInstance = i
}

func (r *rpcInfo) TargetIDC() string {
	r.RLock()
	defer r.RUnlock()
	return r.targetIDC
}

func (r *rpcInfo) SetTargetIDC(s string) {
	r.Lock()
	defer r.Unlock()
	r.targetIDC = s
}

// GetRemoteIP return the IP of remote endpoint.
func (r *rpcInfo) GetRemoteIP(ctx context.Context, resp interface{}) string {
	var remoteIP string

	if ins := r.TargetInstance(); ins != nil {
		remoteIP = ins.Address()
	}

	if kcr, ok := resp.(endpoint.KitcCallResponse); ok && len(kcr.RemoteAddr()) > 0 {
		remoteIP = kcr.RemoteAddr()
	} else {
		if rip, ok := metainfo.GetBackwardValue(ctx, HeaderTransRemoteAddr); ok && rip != "" {
			remoteIP = rip
		}
	}
	return remoteIP
}

func (r *rpcInfo) GetToCluster(ctx context.Context, resp interface{}) string {
	res, _ := metainfo.GetBackwardValue(ctx, HeaderTransToCluster)
	return res
}

type rpcInfoCtxKey struct{}

func GetRPCInfo(ctx context.Context) *rpcInfo {
	return ctx.Value(rpcInfoCtxKey{}).(*rpcInfo)
}

func newCtxWithRPCInfo(ctx context.Context, rpcInfo *rpcInfo) context.Context {
	return context.WithValue(ctx, rpcInfoCtxKey{}, rpcInfo)
}
