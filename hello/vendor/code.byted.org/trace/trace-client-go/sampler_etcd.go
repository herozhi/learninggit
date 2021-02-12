package trace

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"crypto/md5"
	"encoding/json"
	"math/big"

	"code.byted.org/gopkg/logs"
	"code.byted.org/gopkg/tccclient"
	jaeger "code.byted.org/trace/trace-client-go/jaeger-client"
	"code.byted.org/trace/trace-client-go/jaeger-client/thrift-gen/sampling"
	"github.com/pkg/errors"
)

const (
	confTccPsm            = "trace.client.opentracing_sampling"
	centralizedConfTccPsm = "toutiao.infra.opentracing_sampling"
	dyeAppUUIDTccKeyFmt   = "app_%d"

	defaultSamplingRate              = 0.001
	defaultSamplingRefreshInterval   = 3 * time.Minute
	defaultMaxOperations             = 2000
	defaultLowerBoundTracesPerSecond = 1.0 / 1800
)

var (
	defaultSamplingStrategy = fmt.Sprintf(`{
	"service_name": "*",
	"default_strategy": {
		"operation": "*",
		"sampling_rate": %f,

		"rate_limit": %f,
		"max_balance": %f,
		"update_balance_interval": %d,

		"post_spans_rate_limit": {
			"rate_limit": %f,
			"max_balance": %f,
			"update_balance_interval": %d
		}
	},
	"downstream_strategies": {
		"post_spans_rate_limit_max_peers": %d,
		"default": {
			"peer_service": "*",
			"post_spans_rate_limit": {
				"rate_limit": %f,
				"max_balance": %f,
				"update_balance_interval": %d
			}
		}
	}
}`, defaultSamplingRate,
		// span limit
		defaultSampledSpansRateLimitPerSecond,
		defaultSampledSpansRateLimitMaxBalance,
		defaultSampledSpansRateLimitUpdateBalanceInterval,
		// post span limit
		defaultPostSpansRateLimitPerSecond,
		defaultPostSpansRateLimitMaxBalance,
		defaultPostSpansRateLimitUpdateBalanceInterval,
		// downstream max peers
		defaultDownstreamPostSpansRateLimitMaxPeers,
		// downstream post span limit
		defaultDownstreamPostSpansRateLimitPerSecond,
		defaultDownstreamPostSpansRateLimitMaxBalance,
		defaultDownstreamPostSpansRateLimitUpdateBalanceInterval,
	)

	lastSamplingDigest              = md5sum([]byte(defaultSamplingStrategy))
	lastCentralizedSamplingDigest   = md5sum([]byte(defaultSamplingStrategy))
	defaultAdaptiveSamplingStrategy = &sampling.PerOperationSamplingStrategies{
		DefaultSamplingProbability:       defaultSamplingRate,
		DefaultLowerBoundTracesPerSecond: defaultLowerBoundTracesPerSecond,
	}
)

// SamplerOption is a function that sets some option on the sampler
type SamplerOption func(options *samplerOptions)

// SamplerOptions is a factory for all available SamplerOption's
var SamplerOptions samplerOptions

type samplerOptions struct {
	metrics                 *jaeger.Metrics
	maxOperations           int
	sampler                 jaeger.Sampler
	logger                  jaeger.Logger
	samplingRefreshInterval time.Duration
}

// Metrics creates a SamplerOption that initializes Metrics on the sampler,
// which is used to emit statistics.
func (samplerOptions) Metrics(m *jaeger.Metrics) SamplerOption {
	return func(o *samplerOptions) {
		o.metrics = m
	}
}

// MaxOperations creates a SamplerOption that sets the maximum number of
// operations the sampler will keep track of.
func (samplerOptions) MaxOperations(maxOperations int) SamplerOption {
	return func(o *samplerOptions) {
		o.maxOperations = maxOperations
	}
}

// InitialSampler creates a SamplerOption that sets the initial sampler
// to use before a remote sampler is created and used.
func (samplerOptions) InitialSampler(sampler jaeger.Sampler) SamplerOption {
	return func(o *samplerOptions) {
		o.sampler = sampler
	}
}

// Logger creates a SamplerOption that sets the logger used by the sampler.
func (samplerOptions) Logger(logger jaeger.Logger) SamplerOption {
	return func(o *samplerOptions) {
		o.logger = logger
	}
}

// SamplingRefreshInterval creates a SamplerOption that sets how often the
// sampler will poll local agent for the appropriate sampling strategy.
func (samplerOptions) SamplingRefreshInterval(samplingRefreshInterval time.Duration) SamplerOption {
	return func(o *samplerOptions) {
		o.samplingRefreshInterval = samplingRefreshInterval
	}
}

// -----------------------

// EtcdSampler is a delegating sampler that polls from etcd configure center
// for the appropriate sampling strategy, constructs a corresponding sampler and
// delegates to it for sampling decisions.
type EtcdSampler struct {
	// These fields must be first in the struct because `sync/atomic` expects 64-bit alignment.
	// Cf. https://github.com/golang/go/issues/13868
	closed int64 // 0 - not closed, 1 - closed

	sync.RWMutex
	samplerOptions
	serviceName string

	doneChan chan *sync.WaitGroup

	tccConfCli         *tccclient.Client
	centralizedConfCli *tccclient.Client
}

// NewEtcdSampler creates a sampler that periodically pulls
// the sampling strategy from an etcd configure center
func NewEtcdSampler(
	serviceName string,
	opts ...SamplerOption,
) *EtcdSampler {
	options := applySamplerOptions(opts...)
	tccConfCli, err := tccclient.NewClient(confTccPsm, tccclient.NewConfig())
	if err != nil {
		tccConfCli = nil
		logs.Errorf("init tccConfCli failed. ErrMsg: %s", err.Error())
	}

	centralizedConfCli, err := tccclient.NewClient(centralizedConfTccPsm, tccclient.NewConfig())
	if err != nil {
		centralizedConfCli = nil
		logs.Errorf("init centralizedConfCli failed. ErrMsg: %s", err.Error())
	}

	sampler := &EtcdSampler{
		serviceName:        serviceName,
		samplerOptions:     options,
		doneChan:           make(chan *sync.WaitGroup),
		tccConfCli:         tccConfCli,
		centralizedConfCli: centralizedConfCli,
	}

	return sampler
}

func applySamplerOptions(opts ...SamplerOption) samplerOptions {
	options := samplerOptions{}
	for _, option := range opts {
		option(&options)
	}
	if options.logger == nil {
		options.logger = jaeger.NullLogger
	}
	if options.metrics == nil {
		options.metrics = jaeger.NewNullMetrics()
	}
	if options.maxOperations <= 0 {
		options.maxOperations = defaultMaxOperations
	}
	if options.samplingRefreshInterval <= 0 {
		options.samplingRefreshInterval = defaultSamplingRefreshInterval
	}
	if options.sampler == nil {
		options.sampler, _ = jaeger.NewAdaptiveSampler(defaultAdaptiveSamplingStrategy, defaultMaxOperations)
	}

	return options
}

// IsSampled implements IsSampled() of Sampler.
func (s *EtcdSampler) IsSampled(id jaeger.TraceID, operation string) (bool, []jaeger.Tag) {
	s.RLock()
	defer s.RUnlock()
	return s.sampler.IsSampled(id, operation)

}

// Close implements Close() of Sampler.
func (s *EtcdSampler) Close() {
	if swapped := atomic.CompareAndSwapInt64(&s.closed, 0, 1); !swapped {
		s.logger.Error("Repeated attempt to close the sampler is ignored")
		return
	}

	var wg sync.WaitGroup
	wg.Add(1)
	s.doneChan <- &wg
	wg.Wait()
}

// Equal implements Equal() of Sampler.
func (s *EtcdSampler) Equal(other jaeger.Sampler) bool {
	// NB The Equal() function is expensive and will be removed. See adaptiveSampler.Equal() for
	// more information.
	if o, ok := other.(*EtcdSampler); ok {
		s.RLock()
		o.RLock()
		defer s.RUnlock()
		defer o.RUnlock()
		return s.sampler.Equal(o.sampler)
	}

	return false
}

func (s *EtcdSampler) getSampler() jaeger.Sampler {
	s.Lock()
	defer s.Unlock()
	return s.sampler
}

func (s *EtcdSampler) setSampler(sampler jaeger.Sampler) {
	s.Lock()
	defer s.Unlock()
	s.sampler = sampler
}

func dyeAppUUIDTccKey(appid int32) string {
	return fmt.Sprintf(dyeAppUUIDTccKeyFmt, appid)
}

func (s *EtcdSampler) updateSampler() {
	var err error

	if s.tccConfCli != nil {
		registry, oldDyeAppUUIDSet := getAppRegistryAndDyeAppUUIDSet()

		appUUIDValue := make(map[int32]string, len(registry))
		for appid, oldV := range registry {
			v, err := s.tccConfCli.Get(dyeAppUUIDTccKey(appid))
			if err != nil || v == "" {
				s.metrics.DyeAppUUIDQueryFailure.Inc(1)
				appUUIDValue[appid] = oldV
			} else {
				appUUIDValue[appid] = v
				s.metrics.DyeAppUUIDRetrieved.Inc(1)
			}
		}

		dyeAppUUIDSet := make(map[int32]map[int64]bool, len(registry))
		for appid, v := range appUUIDValue {
			if v != registry[appid] {
				var c DyeAppUUIDConf
				if err = json.Unmarshal([]byte(v), &c); err != nil {
					s.logger.Error("json unmarshal failed for app " + fmt.Sprintln(appid))
					s.metrics.DyeAppUUIDUpdateFailure.Inc(1)
					continue
				}
				dyeUUIDSet := make(map[int64]bool)
				for _, uuid := range c.DyeUUIDList {
					dyeUUIDSet[uuid] = true
				}

				dyeAppUUIDSet[appid] = dyeUUIDSet

				s.metrics.DyeAppUUIDUpdated.Inc(1)
			} else {
				dyeAppUUIDSet[appid] = oldDyeAppUUIDSet[appid]
			}
		}

		UpdateDyeAppUUIDSet(dyeAppUUIDSet, appUUIDValue)
	}

	if s.serviceName == "" {
		return
	}

	var psmConfStr, centralizedConfStr string

	var psmConfDigest int64 = 0
	var centralizedConfDigest int64 = 0

	if s.tccConfCli != nil {
		psmConfStr, err = s.tccConfCli.Get(s.serviceName)
		if err != nil || psmConfStr == "" {
			s.metrics.SamplerQueryFailure.Inc(1)
			psmConfStr = defaultSamplingStrategy
			psmConfDigest = lastSamplingDigest
		} else {
			psmConfDigest = md5sum([]byte(psmConfStr))
			s.metrics.SamplerRetrieved.Inc(1)
		}
	}

	if psmConfDigest != lastSamplingDigest {
		s.logger.Infof("digest:%x last-digest:%x json-val:\n%s\n", psmConfDigest, lastSamplingDigest, psmConfStr)
		serviceStrategies, err := jsonToSamplingStrategies(s.serviceName, psmConfStr)
		if err != nil || serviceStrategies == nil {
			s.logger.Error("unmarshal failed. ErrMsg: " + err.Error())
			s.metrics.SamplerUpdateFailure.Inc(1)
		} else {
			if err := s.updateAdaptiveSampler(serviceStrategies); err != nil {
				s.logger.Error("update adaptive sampler failed. ErrMsg: " + err.Error())
				s.metrics.SamplerUpdateFailure.Inc(1)
			} else {
				s.metrics.SamplerUpdated.Inc(1)
				lastSamplingDigest = psmConfDigest
			}
		}
	}

	if s.centralizedConfCli != nil {
		centralizedConfStr, err = s.centralizedConfCli.Get(s.serviceName)
		if err != nil || centralizedConfStr == "" {
			s.metrics.CentralizedSamplerQueryFailure.Inc(1)
			centralizedConfStr = defaultSamplingStrategy
			centralizedConfDigest = lastCentralizedSamplingDigest
		} else {
			centralizedConfDigest = md5sum([]byte(centralizedConfStr))
			s.metrics.CentralizedSamplerRetrieved.Inc(1)
		}
	}

	if centralizedConfDigest != lastCentralizedSamplingDigest {
		s.logger.Infof("digest:%x last-digest:%x json-val:\n%s\n", centralizedConfDigest, lastCentralizedSamplingDigest, centralizedConfStr)
		err := updateCentralizedSamplingStrategies(s.serviceName, centralizedConfStr)
		if err != nil {
			s.logger.Error("updateCentralizedSamplingStrategies failed. ErrMsg: " + err.Error())
			s.metrics.CentralizedSamplerUpdateFailure.Inc(1)
		} else {
			s.metrics.CentralizedSamplerUpdated.Inc(1)
			lastCentralizedSamplingDigest = centralizedConfDigest
		}
	}
}

// NB: this function should only be called while holding a Write lock
func (s *EtcdSampler) updateAdaptiveSampler(strategies *sampling.PerOperationSamplingStrategies) error {
	sampler, err := jaeger.NewAdaptiveSampler(strategies, s.maxOperations)
	if err == nil {
		s.Lock()
		defer s.Unlock()
		s.sampler = sampler
	}
	return err
}

func (s *EtcdSampler) pollController() {
	ticker := time.NewTicker(s.samplingRefreshInterval)
	defer ticker.Stop()
	s.pollControllerWithTicker(ticker)
}

func (s *EtcdSampler) pollControllerWithTicker(ticker *time.Ticker) {
	// update sampler at begin
	s.updateSampler()
	for {
		select {
		case <-ticker.C:
			s.updateSampler()
		case wg := <-s.doneChan:
			wg.Done()
			return
		}
	}
}

func (s *EtcdSampler) start() {
	go s.pollController()
}

type ServiceSamplingStrategy struct {
	ServiceName                string                    `json:"service_name"`
	DefaultStrategy            OperationSamplingParam    `json:"default_strategy"`
	OperationStrategies        []*OperationSamplingParam `json:"operation_strategies"`
	DownstreamStrategies       DownstreamStrategies      `json:"downstream_strategies"`
	RootSpanEnable             *int32                    `json:"root_span_enable"`
	DyeUUIDAppRegistryCapacity *int                      `json:"dye_uuid_app_reg_cap"`
	InnerUUIDList              []int64                   `json:"inner_uuid_list"`
	DyeUUIDList                []int64                   `json:"dye_uuid_list"`
	DebugFromTLBEnable         *int32                    `json:"debug_from_tlb_enable"`
	LowerBoundTracesPerSecond  *float64                  `json:"lower_bound_traces_per_second"`
}

type OperationSamplingParam struct {
	RateLimitParam
	Operation          string         `json:"operation"`
	SamplingRate       *float64       `json:"sampling_rate"`
	PostSpansRateLimit RateLimitParam `json:"post_spans_rate_limit"`
}

type DownstreamStrategies struct {
	PostSpansRateLimitMaxPeers *int `json:"post_spans_rate_limit_max_peers"`

	Default     DownstreamItem    `json:"default"`
	Downstreams []*DownstreamItem `json:"downstreams"`
}

type DownstreamItem struct {
	PeerService        string         `json:"peer_service"`
	PostSpansRateLimit RateLimitParam `json:"post_spans_rate_limit"`
}

type DyeAppUUIDConf struct {
	DyeUUIDList []int64 `json:"dye_uuid_list"`
}

func jsonToSamplingStrategies(serviceName, val string) (res *sampling.PerOperationSamplingStrategies, err error) {
	res, err = nil, nil

	var param ServiceSamplingStrategy
	if err = json.Unmarshal([]byte(val), &param); err != nil {
		err = errors.Errorf("json unmarshal failed for ServiceSamplingStrategy")
		return
	}
	if param.ServiceName != "*" && param.ServiceName != serviceName {
		err = errors.Errorf("service name not match between etcd and local service")
		return
	}

	opsUpperBound := make(map[string]*RateLimitParam)
	opsPostSpansUpperBound := make(map[string]*RateLimitParam)
	downstreamPostSpansUpperBound := make(map[string]*RateLimitParam)
	// construct OperationSamplingStrategy for updating adaptiveSampler
	res = &sampling.PerOperationSamplingStrategies{
		DefaultSamplingProbability:       getFloat64WithDefault(param.DefaultStrategy.SamplingRate, defaultSamplingRate),
		DefaultLowerBoundTracesPerSecond: getFloat64WithDefault(param.LowerBoundTracesPerSecond, defaultLowerBoundTracesPerSecond),
	}
	for _, ops := range param.OperationStrategies {
		normOperation := FormatOperationName(serviceName, ops.Operation)
		res.PerOperationStrategies = append(res.PerOperationStrategies,
			&sampling.OperationSamplingStrategy{
				Operation: normOperation,
				ProbabilisticSampling: &sampling.ProbabilisticSamplingStrategy{
					getFloat64WithDefault(ops.SamplingRate, res.DefaultSamplingProbability),
				},
			})

		if ops.RateLimitParam.IsValid() {
			opsUpperBound[normOperation] = &ops.RateLimitParam
		}
		if ops.PostSpansRateLimit.IsValid() {
			opsPostSpansUpperBound[normOperation] = &ops.PostSpansRateLimit
		}
	}

	for _, ds := range param.DownstreamStrategies.Downstreams {
		if ds.PostSpansRateLimit.IsValid() {
			downstreamPostSpansUpperBound[ds.PeerService] = &ds.PostSpansRateLimit
		}
	}

	sampledSpansUpperBound := defaultSampledSpansRateLimit
	if param.DefaultStrategy.RateLimitParam.IsValid() {
		sampledSpansUpperBound = &param.DefaultStrategy.RateLimitParam
	}
	// update global rate limiter
	globalLimiter.update(0, sampledSpansUpperBound, opsUpperBound)

	postSpansUpperBound := defaultPostSpansRateLimit
	if param.DefaultStrategy.PostSpansRateLimit.IsValid() {
		postSpansUpperBound = &param.DefaultStrategy.PostSpansRateLimit
	}
	globalPostSpansLimiter.update(0, postSpansUpperBound, opsPostSpansUpperBound)

	defaultDownstreamPostSpansUpperBound := defaultDownstreamPostSpansRateLimit
	if param.DownstreamStrategies.Default.PostSpansRateLimit.IsValid() {
		defaultDownstreamPostSpansUpperBound = &param.DownstreamStrategies.Default.PostSpansRateLimit
	}

	downstreamMaxPeers := defaultDownstreamPostSpansRateLimitMaxPeers
	if param.DownstreamStrategies.PostSpansRateLimitMaxPeers != nil {
		downstreamMaxPeers = *param.DownstreamStrategies.PostSpansRateLimitMaxPeers
	}

	downstreamPostSpansLimiter.update(downstreamMaxPeers,
		defaultDownstreamPostSpansUpperBound,
		downstreamPostSpansUpperBound)

	UpdateBytedTracerFromRemoteConf(&param)

	return
}

func md5sum(data []byte) int64 {
	h := md5.New()
	h.Write(data)
	ret := big.NewInt(0)
	ret.SetBytes(h.Sum(nil)[0:8])
	return ret.Int64()
}

func updateCentralizedSamplingStrategies(serviceName, val string) (err error) {

	var param ServiceSamplingStrategy
	if err = json.Unmarshal([]byte(val), &param); err != nil {
		err = errors.Errorf("json unmarshal failed for ServiceSamplingStrategy")
		return
	}

	if len(param.InnerUUIDList) != 0 || len(param.DyeUUIDList) != 0 {
		dyeUUIDSet := make(map[int64]bool)
		for _, uuid := range param.InnerUUIDList {
			dyeUUIDSet[uuid] = true
		}
		for _, uuid := range param.DyeUUIDList {
			dyeUUIDSet[uuid] = true
		}
		UpdateCentralizedDyeUUIDSet(dyeUUIDSet)
	}

	return
}

func getFloat64WithDefault(value *float64, defaultValue float64) float64 {
	if value != nil {
		return *value
	}
	return defaultValue
}
