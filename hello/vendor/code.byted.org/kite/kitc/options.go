package kitc

import (
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"strings"
	"time"

	"code.byted.org/gopkg/thrift"
	"code.byted.org/kite/endpoint"
	"code.byted.org/kite/kitc/connpool"
	"code.byted.org/kite/kitc/discovery"
	"code.byted.org/kite/kitc/loadbalancer"
	"code.byted.org/kite/kitutil/kiterrno"
)

// Option .
type Option struct {
	f func(*Options)
}

// CheckUserError .
type CheckUserError func(resp interface{}) (isFailed bool)

// CheckUSerErrorPair .
type CheckUserErrorPair struct {
	Method  string
	Handler CheckUserError
}

// Options .
type Options struct {
	RPCTimeout       time.Duration
	ReadWriteTimeout time.Duration
	ConnTimeout      time.Duration
	ConnMaxRetryTime time.Duration

	Instances    []*discovery.Instance
	Discoverer   discovery.ServiceDiscoverer
	Loadbalancer loadbalancer.Loadbalancer

	TargetCluster   string
	TargetIDC       string
	TargetIDCConfig []IDCConfig

	UseLongPool    bool
	MaxIdle        int // MaxIdlePerIns
	MaxIdleGlobal  int
	MaxIdleTimeout time.Duration
	ConnPool       connpool.ConnPool

	ServiceCB            CBConfig
	InstanceCB           InstanceCBConfig
	CustomizedCB         CBConfig
	CheckUserErrHandlers map[string]CheckUserError

	DisableRPCLog bool

	UserDefinedMWs           []endpoint.Middleware
	UserDefinedMWsBeforeBase []endpoint.Middleware

	// deprecated
	CircuitBreakerMaxConcurrency int
	IKService                    IKService
	// FIXME: almost are binary(default)
	ProtocolType ProtocolType

	IDCHashFunc   loadbalancer.KeyFunc
	ClusterPolicy *discovery.DiscoveryFilterPolicy
	EnvPolicy     *discovery.DiscoveryFilterPolicy

	//custom max size of TFramedTransport
	MaxFramedSize int32

	MetricsControl kiterrno.MetricsControl

	// Mesh Max Timeout between sdk and proxy
	MaxMeshProxyTimeout time.Duration
	MeshMode            bool
	MeshEgressAddr      string

	AddToHostTagToMetrics bool
}

// Marshal .
func (o *Options) Marshal() map[string]interface{} {
	ret := make(map[string]interface{})
	optTyp := reflect.TypeOf(o).Elem()
	optVal := reflect.ValueOf(o).Elem()
	for i := 0; i < optTyp.NumField(); i++ {
		field := optTyp.Field(i)
		fieldTyp := field.Type
		fieldVal := optVal.Field(i)
		fi := fieldVal.Interface()
		if _, err := json.Marshal(fi); err == nil {
			ret[field.Name] = fi
		} else if fieldTyp.Kind() == reflect.Func {
			ret[field.Name] = getFuncName(fieldVal)
		} else if fieldTyp.Kind() == reflect.Slice {
			elemTyp := fieldTyp.Elem()
			if elemTyp.Kind() == reflect.Func {
				var funcs []string
				for j := 0; j < fieldVal.Len(); j++ {
					funcs = append(funcs, getFuncName(fieldVal.Index(j)))
				}
				ret[field.Name] = funcs
			}
		} else if fieldTyp.Kind() == reflect.Map {
			elemTyp := fieldTyp.Elem()
			if fieldTyp.Key().Kind() == reflect.String && elemTyp.Kind() == reflect.Func {
				funcMap := make(map[string]string)
				for _, key := range fieldVal.MapKeys() {
					funcMap[key.Interface().(string)] = getFuncName(fieldVal.MapIndex(key))
				}
				ret[field.Name] = funcMap
			}
		}
	}
	return ret
}

func newOptions() *Options {
	return &Options{
		MeshMode:       ServiceMeshMode,
		MeshEgressAddr: ServiceMeshEgressAddr,
		MaxFramedSize:  thrift.DEFAULT_MAX_LENGTH,
		MetricsControl: CompatibleMetrics,
		InstanceCB: InstanceCBConfig{ // FIXME: Add to remote config
			IsOpen:      true,
			ErrRate:     0.5,
			MinSample:   200,
			ConseErrors: 300,
		},
	}
}

var globalUserDefinedMWs []endpoint.Middleware // global middleware list, work for all rpc client

// WithMaxMeshProxyTimeout the max timeout between sdk and proxy
func WithMaxMeshProxyTimeout(timeout time.Duration) Option {
	return Option{func(op *Options) {
		op.MaxMeshProxyTimeout = timeout
	}}
}

// WithTimeout config read write timeout
func WithTimeout(timeout time.Duration) Option {
	return Option{func(op *Options) {
		op.ReadWriteTimeout = timeout
		op.RPCTimeout = timeout
		op.MaxMeshProxyTimeout = timeout
	}}
}

// WithConnTimeout config connect timeout, deprecated
func WithConnTimeout(timeout time.Duration) Option {
	return Option{func(op *Options) {
		op.ConnTimeout = timeout
	}}
}

// WithConnMaxRetryTime deprecated
func WithConnMaxRetryTime(d time.Duration) Option {
	return Option{func(op *Options) {
		op.ConnMaxRetryTime = d
	}}
}

// WithLongConnection deprecated, use WithLongPool;
func WithLongConnection(maxIdle int, maxIdleTimeout time.Duration) Option {
	return Option{func(op *Options) {
		// TODO(zhangyuanjia): 因为server端有默认的3s连接超时, 如果空闲连接大于3s, 可能会从连接池里面取出错误连接使用, 故暂时做此限制;
		_magic_maxIdleTimeout := time.Millisecond * 2500
		if maxIdleTimeout > _magic_maxIdleTimeout {
			maxIdleTimeout = _magic_maxIdleTimeout
			fmt.Printf("KITC: maxIdleTimeout is set to limit: 2.5s\n")
		}

		op.UseLongPool = true
		op.MaxIdle = maxIdle
		op.MaxIdleGlobal = 1 << 20 // no limit
		op.MaxIdleTimeout = maxIdleTimeout
	}}
}

// WithLongPool enable the long connection pool feature
// 长连接池的实现方案是每个address对应一个连接池，这个连接池是一个由连接构成的ring, ring的大小为maxIdlePerInstance。
// 当选择好目标地址并需要获取一个连接时，按以下步骤处理:
// (1) 首先尝试从这个ring中获取，如果获取失败(没有空闲连接)，则发起新的连接建立请求，即连接数量可能会超过maxIdlePerInstance
// (2) 如果从ring中获取成功，则检查该连接的空闲时间(自上次放入连接池后)是否超过了maxIdleTimeout, 如果超过则关闭该连接并新建
// (3) 全部成功后返回给上层使用
// 在连接使用完毕准备归还时, 按以下步骤依次处理:
// (1) 检查连接是否正常，如果不正常则直接关闭
// (2) 查看空闲连接是否超过全局的maxIdleGlobal, 如果超过则直接关闭
// (3) 待归还到的连接池的ring中是否还有空闲空间，如果有则直接放入，否则直接关闭
// 从上面可以看出，几个参数的选择考虑如下:
// (1) maxIdlePerInstance 池化的连接数量，最小为1，否则长连接会退化为短连接。具体的值与每个目标地址的吞吐量有关，近似的估算公式为:
//	   maxIdlePerInstance = qps_per_dest_host*avg_response_time_sec。举例如下，假设每个请求的响应时间为100ms,
//	   平摊到每个下游地址的请求为100qps, 该值设置为10(100*0.1), 因为每条连接每秒可以处理10个请求, 100qps需要10个连接进行处理
//	   在实际场景中，也需要考虑到流量的波动。需要特别注意的是，由于maxIdleTimeout最大为2.5s, 即2.5s内该连接没有被使用则会被回收,
//	   在上述例子中，当qps低于: 1/2.5*10=4时，长连接将退化为短连接。
//	   总结: 该值设置较大/较小，都会导致连接复用率低，长连接退化为短连接
// (2) maxIdleGlobal 总的空闲连接数 应大于下游目标总数*maxIdlePerInstance, 超出部分是为了限制未能从连接池中获取连接而主动新建连接的总数量。
//	   备注: 该值存在的价值不大，建议设置为一个较大的值，在后续版本中考虑废弃该参数并提供新的接口
// (3) maxIdleTimeout 连接空闲时间，由于kite server在3s内会清理不活跃的连接，因此client端也需要及时清理空闲较久的连接，避免使用无效的连接，该值
//     通过hardcode上限为2500ms, 建议设置为该值即可
func WithLongPool(maxIdlePerInstance, maxIdleGlobal int, maxIdleTimeout time.Duration) Option {
	return Option{func(op *Options) {
		// TODO(zhangyuanjia): 因为server端有默认的3s连接超时, 如果空闲连接大于3s, 可能会从连接池里面取出错误连接使用, 故暂时做此限制;
		_magic_maxIdleTimeout := time.Millisecond * 2500
		if maxIdleTimeout > _magic_maxIdleTimeout {
			maxIdleTimeout = _magic_maxIdleTimeout
			fmt.Printf("KITC: maxIdleTimeout is set to limit: 2.5s\n")
		}

		op.UseLongPool = true
		op.MaxIdle = maxIdlePerInstance
		op.MaxIdleGlobal = maxIdleGlobal
		op.MaxIdleTimeout = maxIdleTimeout
	}}
}

// WithInstances deprecated, use WithHostPort;
func WithInstances(ins ...*Instance) Option {
	return Option{func(op *Options) {
		dins := make([]*discovery.Instance, 0, len(ins))
		for _, i := range ins {
			dins = append(dins, discovery.NewInstance(i.Host(), i.Port(), i.Tags()))
		}
		op.Instances = dins
	}}
}

// WithHostPort .
func WithHostPort(hosts ...string) Option {
	return Option{func(op *Options) {
		var info = "WithHostPort(" + strings.Join(hosts, ", ") + ")"

		var ins []*discovery.Instance
		for _, hostPort := range hosts {
			host, port, err := net.SplitHostPort(hostPort)
			if err != nil {
				err = fmt.Errorf(info+": %s", err.Error())
				panic(err)
			}
			ins = append(ins, discovery.NewInstance(host, port, nil))
		}
		op.Instances = ins
	}}
}

// WithIKService deprecated, please use WithHostPort
func WithIKService(ikService IKService) Option {
	return Option{func(op *Options) {
		op.IKService = ikService
	}}
}

// WithDiscover .
func WithDiscover(discoverer discovery.ServiceDiscoverer) Option {
	return Option{func(op *Options) {
		op.Discoverer = discoverer
	}}
}

// WithCluster .
func WithCluster(cluster string) Option {
	return Option{func(op *Options) {
		op.TargetCluster = cluster
	}}
}

// WithIDC .
func WithIDC(idc string) Option {
	return Option{func(op *Options) {
		op.TargetIDC = idc
		op.TargetIDCConfig = []IDCConfig{IDCConfig{idc, 100}}
	}}
}

// WithCircuitBreaker .
func WithCircuitBreaker(errRate float64, minSample int, unused int) Option {
	return Option{func(op *Options) {
		op.ServiceCB.IsOpen = true
		op.ServiceCB.ErrRate = errRate
		op.ServiceCB.MinSample = int64(minSample)
	}}
}

// WithInstanceCircuitBreaker .
func WithInstanceCircuitBreaker(errRate float64, minSample int64) Option {
	return Option{func(op *Options) {
		op.InstanceCB.IsOpen = true
		op.InstanceCB.ErrRate = errRate
		op.InstanceCB.MinSample = minSample
	}}
}

// WithInstanceCircuitBreakerV2 根据传入的参数进行判断，分别采用以下三种策略：
// 1. 当样本数 >= minSample 且 错误率 >= errRate
// 2. 当样本数 >= durationSamples 且 连续出错时长 >= duration
// 3. 当连续错误数 >= conseErrors
// 以上三种策略成立任何一种就打开熔断器。
// 如果 duration 为 0 表示不启用策略 2
// 如果 conseErrors 为 0 表示不启用策略 3
func WithInstanceCircuitBreakerV2(errRate float64, minSample int64, duration time.Duration, durationSamples, conseErrors int64) Option {
	return Option{func(op *Options) {
		op.InstanceCB.IsOpen = true
		op.InstanceCB.ErrRate = errRate
		op.InstanceCB.MinSample = minSample

		op.InstanceCB.Duration = duration
		op.InstanceCB.DurationSamples = durationSamples

		op.InstanceCB.ConseErrors = conseErrors
	}}
}

// WithRPCTimeout .
func WithRPCTimeout(timeout time.Duration) Option {
	return Option{func(op *Options) {
		op.RPCTimeout = timeout
	}}
}

// Deprecated. Use WithServiceCircuitbreakerDisabled instead.
func WithDisableCB() Option {
	return WithServiceCircuitbreakerDisabled()
}

// WithServiceCircuitbreakerDisabled disables instance circuitbreaker.
func WithServiceCircuitbreakerDisabled() Option {
	return Option{func(op *Options) {
		op.ServiceCB.IsOpen = false
	}}
}

// WithInstanceCircuitbreakerDisabled disables instance circuitbreaker.
func WithInstanceCircuitbreakerDisabled() Option {
	return Option{func(op *Options) {
		op.InstanceCB.IsOpen = false
	}}
}

// WithLoadbalancer .
func WithLoadbalancer(lb loadbalancer.Loadbalancer) Option {
	return Option{func(op *Options) {
		op.Loadbalancer = lb
	}}
}

// WithDisableRPCLog .
func WithDisableRPCLog() Option {
	return Option{func(op *Options) {
		op.DisableRPCLog = true
	}}
}

// WithMiddleWares will add these MWs after BaseWriterMW
func WithMiddleWares(mws ...endpoint.Middleware) Option {
	return Option{func(op *Options) {
		op.UserDefinedMWs = append(op.UserDefinedMWs, mws...)
	}}
}

// WithMiddleWaresBeforeBase .
func WithMiddleWaresBeforeBase(mws ...endpoint.Middleware) Option {
	return Option{func(op *Options) {
		op.UserDefinedMWsBeforeBase = append(op.UserDefinedMWsBeforeBase, mws...)
	}}
}

// AddGlobalMiddleWares .
func AddGlobalMiddleWares(mws ...endpoint.Middleware) {
	globalUserDefinedMWs = append(globalUserDefinedMWs, mws...)
}

// WithCheckUserErrorHandler .
func WithCheckUserErrorHandler(errRate float64, minSamples int64, pairs ...CheckUserErrorPair) Option {
	return Option{func(op *Options) {
		op.CustomizedCB.IsOpen = true
		op.CustomizedCB.ErrRate = errRate
		op.CustomizedCB.MinSample = minSamples
		if len(pairs) > 0 {
			op.CheckUserErrHandlers = make(map[string]CheckUserError, len(pairs))
		}
		for _, pair := range pairs {
			op.CheckUserErrHandlers[pair.Method] = pair.Handler
		}
	}}
}

// WithProtocolType .
func WithProtocolType(protocolType ProtocolType) Option {
	return Option{func(op *Options) {
		op.ProtocolType = protocolType
	}}
}

// WithHashOnIDC .
func WithHashOnIDC(getKey loadbalancer.KeyFunc) Option {
	return Option{func(op *Options) {
		op.IDCHashFunc = getKey
	}}
}

// WithClusterPolicy .
func WithClusterPolicy(policy *discovery.DiscoveryFilterPolicy) Option {
	return Option{func(op *Options) {
		op.ClusterPolicy = policy
	}}
}

// WithEnvPolicy .
func WithEnvPolicy(policy *discovery.DiscoveryFilterPolicy) Option {
	return Option{func(op *Options) {
		op.EnvPolicy = policy
	}}
}

// WithMaxFramedSize .
func WithMaxFramedSize(maxSize int32) Option {
	return Option{func(op *Options) {
		if maxSize > 0 {
			op.MaxFramedSize = maxSize
		}
	}}
}

// WithMetricsControl provides a way to control the decision of whether certain response
// should be treated as error, and what tags should be output in metrics.
func WithMetricsControl(mc kiterrno.MetricsControl) Option {
	return Option{func(op *Options) {
		if mc == nil {
			panic("Nil is no longer supported. Use kitc.UncompatibleMetrics instead.")
		} else {
			op.MetricsControl = mc
		}
	}}
}

// WithConnPool .
func WithConnPool(cp connpool.ConnPool) Option {
	return Option{func(op *Options) {
		op.ConnPool = cp
	}}
}

// WithToHostTagInMetrics adds to_host tag to metrics.
// CAUTION: This may cause metrics unavailable when there's a lot of downstream instances.
func WithToHostTagInMetrics() Option {
	return Option{func(op *Options) {
		op.AddToHostTagToMetrics = true
	}}
}
