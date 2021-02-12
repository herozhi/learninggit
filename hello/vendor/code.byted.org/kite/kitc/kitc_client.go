package kitc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/logid"
	"code.byted.org/gopkg/logs"
	"code.byted.org/gopkg/metainfo"
	"code.byted.org/kite/endpoint"
	"code.byted.org/kite/kitc/discovery"
	"code.byted.org/kite/kitc/loadbalancer"
	"code.byted.org/kite/kitutil"
	"code.byted.org/kite/kitutil/kiterrno"
	"code.byted.org/kite/kitutil/kitevent"
	"code.byted.org/kite/kitutil/kvstore"
)

const (
	// Keys for middleware builder context
	KITC_EVENT_BUS_KEY = "K_EVENT_BUS"
	KITC_OPTIONS_KEY   = "K_KITC_OPTIONS"
	KITC_INSPECTOR     = "K_INSPECTOR"

	// Keys for reuqest context
	KITC_CLIENT_KEY      = "K_KITC_CLIENT" // KitcClient指针
	KITC_RAW_REQUEST_KEY = "K_RAW_REQUEST"

	KITECLIENTKEY = KITC_CLIENT_KEY // for compatibility
)

// KitcClient ...
type KitcClient struct {
	name        string
	client      Client
	chain       endpoint.Middleware
	once        sync.Once
	eventQueues []kitevent.Queue
	eventBus    kitevent.EventBus
	inspector   kitutil.Inspector

	opts           *Options
	remoteConfiger RemoteConfiger
}

// NewClient ...
func NewClient(name string, ops ...Option) (*KitcClient, error) {
	client, ok := clients[name]
	if !ok {
		return nil, fmt.Errorf("Unknow client name %s, forget import?", name)
	}
	return NewWithThriftClient(name, client, ops...)
}

// NewWithThriftClient ...
func NewWithThriftClient(name string, thriftClient Client, ops ...Option) (*KitcClient, error) {
	eventBus := kitevent.NewEventBus()
	inspector := kitutil.NewInspector()
	eventQueues := installEventListener(eventBus, inspector)

	opts := newOptions()
	for _, do := range ops {
		do.f(opts)
	}

	kitclient := &KitcClient{
		name:        name,
		opts:        opts,
		client:      thriftClient,
		eventQueues: eventQueues,
		eventBus:    eventBus,
		inspector:   inspector,
	}

	if opts.MeshMode {
		kitclient.remoteConfiger = newMeshConfiger(eventBus, kvstore.NewETCDStorer())
	} else {
		kitclient.remoteConfiger = newRemoteConfiger(eventBus, kvstore.NewETCDStorer())
	}

	registerKitcClient(kitclient)
	return kitclient, nil
}

func (kc *KitcClient) SetChain(chain endpoint.Middleware) {
	kc.chain = chain
}

func (kc *KitcClient) initMWChain() {
	if kc.chain != nil {
		return
	}

	// Context for middleware initialization
	mwCtx := context.Background()
	mwCtx = context.WithValue(mwCtx, KITC_EVENT_BUS_KEY, kc.eventBus)
	mwCtx = context.WithValue(mwCtx, KITC_INSPECTOR, kc.inspector)
	mwCtx = context.WithValue(mwCtx, KITC_OPTIONS_KEY, kc.opts)
	mwCtx = context.WithValue(mwCtx, KITC_CLIENT_KEY, kc)

	var mids []endpoint.Middleware

	mids = append(mids, globalUserDefinedMWs...)
	mids = append(mids, kc.opts.UserDefinedMWsBeforeBase...)
	mids = append(mids, BaseWriterMW, OpenTracingMW)
	mids = append(mids, kc.opts.UserDefinedMWs...)
	mids = append(mids, NewRPCLogMW(mwCtx))
	mids = append(mids, RPCMetricsMW(mwCtx))

	if kc.opts.MeshMode {
		mids = append(mids,
			NewUserErrorCBMW(mwCtx),
			MeshRPCTimeoutMW,
			NewMeshPoolMW(mwCtx),
			MeshIOErrorHandlerMW,
			MeshSetHeadersMW,
		)
	} else {
		mids = append(mids,
			RPCACLMW,
			StressBotMW,
			DegradationMW,
			NewUserErrorCBMW(mwCtx),
			NewServiceBreakerMW(mwCtx),
			RPCTimeoutMW,
			NewIDCSelectorMW(mwCtx),
			NewServiceDiscoverMW(mwCtx),
			NewLoadbalanceMW(mwCtx),
			NewInstanceBreakerMW(mwCtx),
			NewPoolMW(mwCtx),
			IOErrorHandlerMW,
		)
	}

	kc.chain = endpoint.Chain(mids[0], mids[1:]...)
}

func (kc *KitcClient) MethodInit(method string, ctx context.Context, request interface{}) (context.Context, error) {
	metricsClient.EmitCounter("kite.request.throughput", 1)

	// MWs依赖的某些外部组件, 需要在运行时才能确定, 所以在第一次Call时, 进行初始化
	kc.once.Do(kc.initMWChain)

	// 因为request会被生成代码包裹,
	// 如果用户使用了自定义的LB, 需要保证直接把最原始的req传递给用户
	ctx = context.WithValue(ctx, KITC_RAW_REQUEST_KEY, request)

	return kc.initRPCInfo(method, ctx)
}

func (kc *KitcClient) MethodCall(next endpoint.EndPoint, ctx context.Context, request interface{}) (endpoint.KitcCallResponse, error) {
	ctx = context.WithValue(ctx, KITC_CLIENT_KEY, kc)
	resp, err := kc.chain(next)(ctx, request)
	if _, ok := resp.(endpoint.KitcCallResponse); !ok {
		if err == nil {
			return nil, kiterrno.NewKiteError(kiterrno.KITE, kiterrno.ReturnNilRespNilErrCode, nil)
		}
		return nil, err
	}
	return resp.(endpoint.KitcCallResponse), err
}

// Call do some remote calling
func (kc *KitcClient) Call(method string, ctx context.Context, request interface{}) (endpoint.KitcCallResponse, error) {
	ctx, err := kc.MethodInit(method, ctx, request)
	if err != nil {
		return nil, err
	}
	caller := kc.client.New(kc)
	next, request := caller.Call(method, request)
	return kc.MethodCall(next, ctx, request)
}

func (kc *KitcClient) initRPCInfo(method string, ctx context.Context) (context.Context, error) {
	// construct RPCMeta
	toCluster := kitutil.GetCtxWithDefault(kitutil.GetCtxTargetClusterName, ctx, kc.opts.TargetCluster)
	if toCluster == "" && !kc.opts.MeshMode {
		toCluster = "default"
	}

	rpcMeta := RPCMeta{
		From:        kitutil.GetCtxWithDefault(kitutil.GetCtxServiceName, ctx, env.PSM()),
		FromCluster: kitutil.GetCtxWithDefault(kitutil.GetCtxCluster, ctx, env.Cluster()),
		FromMethod:  kitutil.GetCtxWithDefault(kitutil.GetCtxMethod, ctx, "unknown_method"),
		To:          kc.Name(),
		ToCluster:   toCluster,
		Method:      method,
	}
	if rpcMeta.From == "" {
		return nil, errors.New("no service's name for rpc call, you can use kitutil.NewCtxWithServiceName(ctx, xxxx) to set ctx")
	}

	// get RPCConf
	rpcConf, err := kc.GetRPCConfig(rpcMeta)
	if err != nil {
		return nil, fmt.Errorf("get RPC config err: %s", err.Error())
	}

	// TODO(zhangyuanjia): 移除该兼容性字段
	rpcTimeout, _ := kitutil.GetCtxRPCTimeout(ctx)
	if rpcTimeout > 0 {
		rpcConf.RPCTimeout = int(rpcTimeout / time.Millisecond)
	}

	// prepare some extra fields
	if logID, ok := kitutil.GetCtxLogID(ctx); !ok || logID == "" {
		logID = logid.GenLogID()
		ctx = kitutil.NewCtxWithLogID(ctx, logID)
	}
	instances, _ := kitutil.GetCtxRPCInstances(ctx)

	// construct RPCInfo
	rpcInfo := &rpcInfo{
		RPCMeta:   rpcMeta,
		RPCConfig: rpcConf,
		LogID:     kitutil.GetCtxWithDefault(kitutil.GetCtxLogID, ctx, ""),
		LocalIP:   kitutil.GetCtxWithDefault(kitutil.GetCtxLocalIP, ctx, env.HostIP()),
		Env:       kitutil.GetCtxWithDefault(kitutil.GetCtxEnv, ctx, ""),
		StressTag: kitutil.GetCtxWithDefault(kitutil.GetCtxStressTag, ctx, ""),
		TraceTag:  kitutil.GetCtxWithDefault(kitutil.GetCtxTraceTag, ctx, ""),
		instances: instances,
	}

	if ServiceMeshMode {
		var host, port string
		var tags map[string]string
		if strings.Contains(kc.opts.MeshEgressAddr, ":") {
			host, port, err = net.SplitHostPort(kc.opts.MeshEgressAddr)
			if err != nil {
				return nil, err
			}
		} else {
			host = kc.opts.MeshEgressAddr // domain socket mode
			tags = map[string]string{"network": "unix"}
		}
		rpcInfo.targetInstance = &discovery.Instance{
			Host: host,
			Port: port,
			Tags: tags,
		}

		// dest_address
		if len(rpcInfo.instances) == 0 {
			rpcInfo.instances = kc.opts.Instances
		}
		rpcInfo.targetIDC = kc.opts.TargetIDC

		if consist, ok := kc.opts.Loadbalancer.(*loadbalancer.ConsistBalancer); ok {
			req := ctx.Value(KITC_RAW_REQUEST_KEY)
			if ringHashKey, err := consist.GetKey(ctx, req); err == nil && len(ringHashKey) > 0 {
				ctx = context.WithValue(ctx, RingHashKeyType(":CH"), ringHashKey)
				rpcInfo.RingHashKey = ringHashKey
			}
		}
	}

	// Setup for sending reponse headers back from later processes.
	ctx = metainfo.WithBackwardValues(ctx)
	return newCtxWithRPCInfo(ctx, rpcInfo), nil
}

// Name .
func (kc *KitcClient) Name() string {
	return kc.name
}

// GetRPCConfig .
// TODO(zhengjianbo): RPCMeta, RPCConfig使用指针，避免多次copy, 需要注意defaultRPCConfig的deepcopy
func (kc *KitcClient) GetRPCConfig(r RPCMeta) (RPCConfig, error) {
	c, err := kc.remoteConfiger.GetRemoteConfig(r)
	if err != nil {
		logs.Warnf("KITC: get remote config err: %s, default config will be used", err.Error())
		c = defaultRPCConfig
	}

	// merge options' config
	if kc.opts.ReadWriteTimeout > 0 {
		timeoutInMS := int(kc.opts.ReadWriteTimeout / time.Millisecond)
		c.ReadTimeout = timeoutInMS
		c.WriteTimeout = timeoutInMS
		c.RPCTimeout = timeoutInMS
	}
	if kc.opts.ConnTimeout > 0 {
		timeoutInMS := int(kc.opts.ConnTimeout / time.Millisecond)
		c.ConnectTimeout = timeoutInMS
	}
	if kc.opts.RPCTimeout > 0 {
		timeoutInMS := int(kc.opts.RPCTimeout / time.Millisecond)
		c.ReadTimeout = timeoutInMS
		c.WriteTimeout = timeoutInMS
		c.RPCTimeout = timeoutInMS
	}

	if kc.opts.ServiceCB.Valid() {
		c.ServiceCB = kc.opts.ServiceCB
	}
	if kc.opts.TargetIDC != "" && len(kc.opts.TargetIDCConfig) > 0 {
		c.IDCConfig = kc.opts.TargetIDCConfig
	}

	return c, nil
}

// Options .
func (kc *KitcClient) Options() *Options {
	return kc.opts
}

// RemoteConfigs .
func (kc *KitcClient) RemoteConfigs() map[string]RPCConfig {
	return kc.remoteConfiger.GetAllRemoteConfigs()
}
