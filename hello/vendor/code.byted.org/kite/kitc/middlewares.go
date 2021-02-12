package kitc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"runtime"
	"sort"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash"
	"github.com/opentracing/opentracing-go"

	circuit "code.byted.org/gopkg/circuitbreaker"
	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/logs"
	"code.byted.org/gopkg/metainfo"
	"code.byted.org/gopkg/metrics"
	"code.byted.org/gopkg/rand"
	"code.byted.org/gopkg/thrift"
	"code.byted.org/kite/endpoint"
	"code.byted.org/kite/kitc/connpool"
	"code.byted.org/kite/kitc/discovery"
	"code.byted.org/kite/kitc/loadbalancer"
	"code.byted.org/kite/kitutil"
	"code.byted.org/kite/kitutil/kiterrno"
	"code.byted.org/kite/kitutil/kitevent"
	"code.byted.org/trace/trace-client-go"
	kext "code.byted.org/trace/trace-client-go/ext"
)

// clientBase implement endpoint.BaseInterface
type clientBase struct {
	logID   string
	caller  string
	client  string
	addr    string
	env     string
	cluster string
}

// GetLogID return logid
func (cb *clientBase) GetLogID() string {
	return cb.logID
}

// GetCaller return caller
func (cb *clientBase) GetCaller() string {
	return cb.caller
}

// GetClient return client
func (cb *clientBase) GetClient() string {
	return cb.client
}

// GetAddr return addr
func (cb *clientBase) GetAddr() string {
	return cb.addr
}

// GetEnv return this request's env
func (cb *clientBase) GetEnv() string {
	return cb.env
}

// GetCluster return upstream's cluster's name
func (cb *clientBase) GetCluster() string {
	return cb.cluster
}

func (cb *clientBase) GetExtra() map[string]string {
	// no-op
	return nil
}

// BaseWriterMW write base info to request
func BaseWriterMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req, ok := request.(endpoint.KitcCallRequest)
		if !ok {
			logs.CtxWarn(ctx, "request %v not implement KitcCallRequest", request)
			return next(ctx, request)
		}
		rpcInfo := GetRPCInfo(ctx)

		req.SetBase(&clientBase{
			logID:   rpcInfo.LogID,
			caller:  rpcInfo.From,
			client:  rpcInfo.Client,
			addr:    rpcInfo.LocalIP,
			cluster: rpcInfo.FromCluster,
			env:     rpcInfo.Env,
		})

		realRequest := req.RealRequest()

		extraInfo := make(map[string]string)
		if rpcInfo.StressTag != "" {
			extraInfo["stress_tag"] = rpcInfo.StressTag
		}

		userExtra := make(map[string]string)
		if ue, ok := kitutil.GetCtxDownstreamUserExtra(ctx); ok && len(ue) > 0 {
			userExtra = ue
		}
		metainfo.SaveMetaInfoToMap(ctx, userExtra)
		if len(userExtra) > 0 {
			if extraStr, err := kitutil.Map2JSONStr(userExtra); err == nil {
				extraInfo["user_extra"] = string(extraStr)
			} else {
				logs.CtxError(ctx, "set user extra err: %s", err.Error())
			}
		}
		if err := hackExtra(realRequest, extraInfo); err != nil {
			logs.CtxWarn(ctx, "set user extra err: %s", err.Error())
		}

		return next(ctx, req)
	}
}

func hackExtra(req interface{}, kvs map[string]string) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("%v", e)
		}
	}()

	ptr := reflect.ValueOf(req)
	for ptr.Kind() == reflect.Ptr {
		ptr = ptr.Elem()
	}
	if ptr.Kind() != reflect.Struct {
		return nil
	}
	// get Base
	basePtr := ptr.FieldByName("Base")
	for basePtr.Kind() == reflect.Ptr {
		basePtr = basePtr.Elem()
	}
	if basePtr.Kind() != reflect.Struct {
		return nil
	}

	// get Extra
	extra := basePtr.FieldByName("Extra")
	if !extra.CanSet() {
		return nil
	}
	origin, ok := extra.Interface().(map[string]string)
	if !ok {
		return fmt.Errorf("kite set base.Extra failed")
	}
	for k := range origin {
		if _, ok := kvs[k]; !ok {
			kvs[k] = origin[k]
		}
	}
	extra.Set(reflect.ValueOf(kvs))
	return nil
}

// selectIDC randomly picks a IDC from `idcConfig`.
func selectIDC(idcConfig []IDCConfig, hashCode *uint64) string {
	var sum, rd int
	for _, pl := range idcConfig {
		sum += pl.Percent
	}

	if sum == 0 {
		return env.IDC()
	}

	if hashCode != nil {
		sort.Slice(idcConfig, func(i, j int) bool {
			return idcConfig[i].IDC < idcConfig[j].IDC
		})
		rd = int(*hashCode % uint64(sum))
	} else {
		rd = rand.Intn(sum)
	}

	for _, pl := range idcConfig {
		if rd < pl.Percent {
			return pl.IDC
		}
		rd -= pl.Percent
	}
	return idcConfig[0].IDC
}

// NewIDCSelectorMW creates a middleware doing IDC selection.
func NewIDCSelectorMW(mwCtx context.Context) endpoint.Middleware {
	opts := mwCtx.Value(KITC_OPTIONS_KEY).(*Options)

	var GetKey loadbalancer.KeyFunc = opts.IDCHashFunc
	if GetKey == nil {
		return func(next endpoint.EndPoint) endpoint.EndPoint {
			return func(ctx context.Context, request interface{}) (interface{}, error) {
				rpcInfo := GetRPCInfo(ctx)
				idcConfig := rpcInfo.IDCConfig
				rpcInfo.SetTargetIDC(selectIDC(idcConfig, nil))
				return next(ctx, request)
			}
		}
	}
	return func(next endpoint.EndPoint) endpoint.EndPoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			rpcInfo := GetRPCInfo(ctx)
			idcConfig := rpcInfo.IDCConfig

			var lbRequest = ctx.Value(KITC_RAW_REQUEST_KEY)

			if hashKey, err := GetKey(ctx, lbRequest); err != nil {
				rpcInfo.SetTargetIDC(selectIDC(idcConfig, nil))
			} else {
				hashCode := xxhash.Sum64String(hashKey)
				rpcInfo.SetTargetIDC(selectIDC(idcConfig, &hashCode))
			}
			return next(ctx, request)
		}
	}
}

// IOErrorHandlerMW .
func IOErrorHandlerMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		rpcInfo := GetRPCInfo(ctx)
		defer func(begin time.Time) {
			tags := []metrics.T{
				{"to", rpcInfo.To},
				{"method", rpcInfo.Method},
				{"from_cluster", rpcInfo.FromCluster},
				{"to_cluster", rpcInfo.ToCluster},
			}
			mname := fmt.Sprintf("service.thrift.%s.call.thrift.latency.us", rpcInfo.From)
			cost := time.Since(begin).Nanoseconds() / 1000 //us
			metricsClient.EmitTimer(mname, cost, tags...)
		}(time.Now())

		span := opentracing.SpanFromContext(ctx)
		if trace.IsSampled(span) {
			span.LogFields(kext.EventKindPkgSendStart)
		}
		resp, err := next(ctx, request)
		if err == nil {
			if trace.IsSampled(span) {
				span.LogFields(kext.EventKindPkgRecvEnd)
			}
			return resp, nil
		}

		if terr, ok := err.(thrift.TApplicationException); ok {
			errMsg := terr.Error()

			// restore the error information from kite server side
			if kerr := kiterrno.ParseErrorMessage(errMsg); kerr != nil {
				return nil, kerr
			}

			// recognized as mesh proxy error
			typeID := int(terr.TypeId())
			if kiterrno.IsMeshErrorCode(typeID) {
				me := kiterrno.NewKiteError(kiterrno.MESH, typeID, terr)
				return nil, me
			}
		}
		// native thrift error will be wrapped as RemoteOrNetworkError
		return nil, kiterrno.NewKiteError(kiterrno.KITE, kiterrno.RemoteOrNetErrCode, err)
	}
}

func makeTimeoutErr(ctx context.Context, rpcInfo *rpcInfo, start time.Time) error {
	var ctxErr string
	if ctx.Err() == context.Canceled {
		ctxErr = "context canceled by business."
	}

	timeout := time.Duration(rpcInfo.RPCTimeout) * time.Millisecond

	if ddl, ok := ctx.Deadline(); !ok {
		ctxErr = "unknown error: context deadline not set?"
	} else {
		if ddl.Before(start.Add(timeout)) {
			ctxErr = fmt.Sprintf("context deadline set by business, expected timeout sub context deadline: %v, context deadline sub start: %v", start.Add(timeout).Sub(ddl), ddl.Sub(start))
		}
	}

	errMsg := fmt.Sprintf("timeout=%v", timeout)
	if ctxErr != "" {
		errMsg = fmt.Sprintf("%s, %s", errMsg, ctxErr)
	}
	return kiterrno.NewKiteError(kiterrno.KITE, kiterrno.RPCTimeoutCode, errors.New(errMsg))
}

// RPCTimeoutMW .
func RPCTimeoutMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		rpcInfo := GetRPCInfo(ctx)
		start := time.Now()
		ctx, cancel := context.WithTimeout(ctx, time.Duration(rpcInfo.RPCTimeout)*time.Millisecond)
		defer cancel()

		var resp interface{}
		var err error
		done := make(chan error, 1)
		go func() {
			defer func() {
				if err := recover(); err != nil {
					const size = 64 << 10
					buf := make([]byte, size)
					buf = buf[:runtime.Stack(buf, false)]

					logs.CtxError(ctx, "KITC: panic: Request: %v, err: %v\n%s", request, err, buf)
					done <- fmt.Errorf("KITC: panic, %v\n%s", err, buf)
				}
				close(done)
			}()

			resp, err = next(ctx, request)
		}()

		select {
		case panicErr := <-done:
			if panicErr != nil {
				panic(panicErr.Error()) // throws panic error
			}
			return resp, err
		case <-ctx.Done():
			return nil, makeTimeoutErr(ctx, rpcInfo, start)
		}
	}
}

// DegradationMW .
func DegradationMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		rpcInfo := GetRPCInfo(ctx)
		if rpcInfo.DegraPercent <= 0 {
			return next(ctx, request)
		}
		if rpcInfo.DegraPercent >= 100 {
			kerr := kiterrno.NewKiteError(kiterrno.KITE, kiterrno.ForbiddenByDegradationCode, nil)
			return nil, kerr
		}

		per := rand.Intn(101)
		if per < rpcInfo.DegraPercent {
			kerr := kiterrno.NewKiteError(kiterrno.KITE, kiterrno.ForbiddenByDegradationCode, nil)
			return nil, kerr
		}

		return next(ctx, request)
	}
}

func handlePoolEvent(ctx context.Context, pool connpool.ConnPool) {
	if longPool, ok := pool.(connpool.LongConnPool); ok {
		// Clean long connections when instances removed by service discovery changes
		handler := func(event *kitevent.KitEvent) {
			deletedIns := event.Extra["deleted"].([]*discovery.Instance)
			for _, ins := range deletedIns {
				longPool.Clean(ins.Network(), ins.Address())
			}
		}

		ebus := ctx.Value(KITC_EVENT_BUS_KEY).(kitevent.EventBus)
		ebus.Watch(SERVICE_ADDRESS_CHANGE, handler)
	}
}

func newPoolFromCtx(ctx context.Context) connpool.ConnPool {
	opts := ctx.Value(KITC_OPTIONS_KEY).(*Options)
	name := ctx.Value(KITC_CLIENT_KEY).(*KitcClient).Name()

	var pool connpool.ConnPool
	if opts.ConnPool != nil {
		pool = opts.ConnPool
	} else if opts.UseLongPool {
		pool = connpool.NewLongPool(opts.MaxIdle, opts.MaxIdleGlobal, opts.MaxIdleTimeout, name)
	} else {
		pool = connpool.NewShortPool(name)
	}

	handlePoolEvent(ctx, pool)
	return pool
}

// NewPoolMW .
func NewPoolMW(mwCtx context.Context) endpoint.Middleware {
	var pool connpool.ConnPool = newPoolFromCtx(mwCtx)

	return func(next endpoint.EndPoint) endpoint.EndPoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			rpcInfo := GetRPCInfo(ctx)
			begin := time.Now()

			span := opentracing.SpanFromContext(ctx)
			if trace.IsSampled(span) {
				span.LogFields(kext.EventKindConnectStart)
			}

			ins := rpcInfo.TargetInstance()
			conn, err := pool.Get(
				ins.Network(),
				ins.Address(),
				time.Duration(rpcInfo.ConnectTimeout)*time.Millisecond)
			cost := int32(time.Now().Sub(begin) / time.Microsecond)
			atomic.StoreInt32(&rpcInfo.ConnCost, cost)

			if trace.IsSampled(span) {
				span.LogFields(kext.EventKindConnectEnd)
			}

			if err != nil {
				kerr := kiterrno.NewKiteError(kiterrno.KITE, kiterrno.GetConnErrorCode, err)
				return nil, kerr
			}

			rpcInfo.SetConn(&connpool.ConnWithPkgSize{
				Conn: conn,
			})
			resp, err := next(ctx, request)

			if err == nil {
				pool.Put(conn)
			} else {
				pool.Discard(conn)
			}
			return resp, err
		}
	}
}

// RPCACLMW .
func RPCACLMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		rpcInfo := GetRPCInfo(ctx)
		if !rpcInfo.ACLAllow {
			aclErr := kiterrno.NewKiteError(kiterrno.KITE, kiterrno.NotAllowedByACLCode, nil)
			return nil, aclErr
		}

		return next(ctx, request)
	}
}

// StressBotMW .
func StressBotMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		r := GetRPCInfo(ctx)
		if !r.StressBotSwitch && r.StressTag != "" {
			err := fmt.Errorf("to=%s, cluster=%s, stress_tag=%s", r.To, r.ToCluster, r.StressTag)
			kerr := kiterrno.NewKiteError(kiterrno.KITE, kiterrno.StressBotRejectionCode, err)
			return nil, kerr
		}
		return next(ctx, request)
	}
}

func cbDebugInfo(breakerPanel circuit.Panel) map[string]interface{} {
	cbMap := make(map[string]interface{})
	for service, breaker := range breakerPanel.DumpBreakers() {
		cbMap[service] = map[string]interface{}{
			"state":             breaker.State().String(),
			"successes in 10s":  breaker.Metricer().Successes(),
			"failures in 10s":   breaker.Metricer().Failures(),
			"timeouts in 10s":   breaker.Metricer().Timeouts(),
			"error rate in 10s": breaker.Metricer().ErrorRate(),
		}
	}
	return cbMap
}

func newCBHandler(name string, eBus kitevent.EventBus) circuit.PanelStateChangeHandler {
	return func(key string, oldState, newState circuit.State, m circuit.Metricer) {
		successes, failures, timeouts := m.Counts()
		var errRate float64
		if (successes + failures + timeouts) == 0 {
			errRate = 0.0
		}

		errRate = float64(failures+timeouts) / float64(successes+failures+timeouts)
		e := &kitevent.KitEvent{
			Name: name,
			Time: time.Now(),
			Detail: fmt.Sprintf("%s: %s -> %s, (succ: %d, err: %d, tmout: %d, rate: %f)",
				key, oldState, newState, successes, failures, timeouts, errRate),
		}
		eBus.Dispatch(e)
	}
}

// NewInstanceBreakerMW .
func NewInstanceBreakerMW(mwCtx context.Context) endpoint.Middleware {
	ebus := mwCtx.Value(KITC_EVENT_BUS_KEY).(kitevent.EventBus)
	insp := mwCtx.Value(KITC_INSPECTOR).(kitutil.Inspector)
	opts := mwCtx.Value(KITC_OPTIONS_KEY).(*Options)

	if !(opts.InstanceCB.IsOpen && opts.InstanceCB.Valid()) {
		return endpoint.EmptyMiddleware
	}

	breakerPanel, _ := circuit.NewPanel(newCBHandler(INSTANCE_CB_CHANGE, ebus), circuit.Options{
		ShouldTrip: circuit.ConsecutiveTripFuncV2(opts.InstanceCB.ErrRate, opts.InstanceCB.MinSample,
			opts.InstanceCB.Duration, opts.InstanceCB.DurationSamples,
			opts.InstanceCB.ConseErrors),
	})

	ebus.Watch(SERVICE_ADDRESS_CHANGE, func(event *kitevent.KitEvent) {
		deletedIns := event.Extra["deleted"].([]*discovery.Instance)
		for _, ins := range deletedIns {
			breakerPanel.RemoveBreaker(ins.Address())
		}
	})

	insp.Register(func(data map[string]interface{}) {
		data["ip_circuitbreaker"] = cbDebugInfo(breakerPanel)
	})

	return func(next endpoint.EndPoint) endpoint.EndPoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			rpcInfo := GetRPCInfo(ctx)
			key := rpcInfo.TargetInstance().Address()

			if !breakerPanel.IsAllowed(key) {
				err := fmt.Errorf("service=%s method=%s hostport=%s", rpcInfo.To, rpcInfo.Method, key)
				kerr := kiterrno.NewKiteError(kiterrno.KITE, kiterrno.NotAllowedByInstanceCBCode, err)
				return nil, kerr
			}

			resp, err := next(ctx, request)
			if err == nil {
				breakerPanel.Succeed(key)
				return resp, err
			}

			kerr, ok := err.(kiterrno.KiteError)
			if !ok || kerr.Category() != kiterrno.KITE {
				// TODO: should we really ignore this error ?
				return resp, err
			}

			// we only concern about connection error
			switch kerr.Errno() {
			case kiterrno.GetConnErrorCode, kiterrno.RemoteOrNetErrCode:
				breakerPanel.Fail(key)
			default:
				breakerPanel.Succeed(key)
			}
			return resp, err
		}
	}
}

// NewUserErrorCBMW creates a middleware to perform circuitbreak with customized judging function.
func NewUserErrorCBMW(mwCtx context.Context) endpoint.Middleware {
	var checkUserErrHandlers map[string]CheckUserError
	var breakerPanel circuit.Panel

	ebus := mwCtx.Value(KITC_EVENT_BUS_KEY).(kitevent.EventBus)
	opts := mwCtx.Value(KITC_OPTIONS_KEY).(*Options)
	if len(opts.CheckUserErrHandlers) == 0 {
		return endpoint.EmptyMiddleware
	}

	breakerPanel, _ = circuit.NewPanel(newCBHandler(USER_ERR_CB_CHANGE, ebus), circuit.Options{
		ShouldTrip: circuit.RateTripFunc(opts.CustomizedCB.ErrRate, opts.CustomizedCB.MinSample),
	})
	checkUserErrHandlers = opts.CheckUserErrHandlers

	return func(next endpoint.EndPoint) endpoint.EndPoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			rpcInfo := GetRPCInfo(ctx)
			cbKey := rpcInfo.To + ":" + rpcInfo.ToCluster + ":" + rpcInfo.Method
			if !breakerPanel.IsAllowed(cbKey) {
				err := fmt.Errorf("service:cluter:method=%s", cbKey)
				kerr := kiterrno.NewKiteError(kiterrno.KITE, kiterrno.NotAllowedByUserErrCBCode, err)
				return nil, kerr
			}

			resp, err := next(ctx, request)
			if err == nil && checkUserErrHandlers[rpcInfo.Method] != nil {
				if !checkUserErrHandlers[rpcInfo.Method](resp) {
					breakerPanel.Succeed(cbKey)
				} else {
					breakerPanel.Fail(cbKey)
				}
			}
			return resp, err
		}
	}
}

// NewServiceBreakerMW .
func NewServiceBreakerMW(mwCtx context.Context) endpoint.Middleware {
	ebus := mwCtx.Value(KITC_EVENT_BUS_KEY).(kitevent.EventBus)
	insp := mwCtx.Value(KITC_INSPECTOR).(kitutil.Inspector)

	breakerPanel, _ := circuit.NewPanel(newCBHandler(SERVICE_CB_CHANGE, ebus), circuit.Options{})

	insp.Register(func(data map[string]interface{}) {
		data["service_circuitbreaker"] = cbDebugInfo(breakerPanel)
	})

	return func(next endpoint.EndPoint) endpoint.EndPoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			rpcInfo := GetRPCInfo(ctx)
			conf := rpcInfo.ServiceCB

			if !conf.IsOpen {
				return next(ctx, request)
			}

			cbKey := rpcInfo.To + ":" + rpcInfo.ToCluster + ":" + rpcInfo.Method
			if !breakerPanel.IsAllowed(cbKey) {
				err := fmt.Errorf("service:cluster:method=%s", cbKey)
				kerr := kiterrno.NewKiteError(kiterrno.KITE, kiterrno.NotAllowedByServiceCBCode, err)
				return nil, kerr
			}

			resp, err := next(ctx, request)
			if err == nil { // succeed
				breakerPanel.Succeed(cbKey)
				return resp, err
			}

			kerr, ok := err.(kiterrno.KiteError)
			if !ok || kerr.Category() != kiterrno.KITE {
				breakerPanel.Succeed(cbKey)
				return resp, err
			}

			switch kerr.Errno() {
			// ignore all internal errors(like NoExpectedField, IDCSelectError) and
			// all ACL and degradation errors, and
			// all RPC timeout errors which have already been recored when the MW receive this error;
			case kiterrno.NotAllowedByACLCode,
				kiterrno.ForbiddenByDegradationCode,
				kiterrno.GetDegradationPercentErrorCode,
				kiterrno.BadConnBalancerCode,
				kiterrno.BadConnRetrierCode,
				kiterrno.ServiceDiscoverCode:
			// regard all network errors and relative errors caused by network as failed
			case kiterrno.NotAllowedByInstanceCBCode,
				kiterrno.ConnRetryCode,
				kiterrno.GetConnErrorCode,
				kiterrno.RemoteOrNetErrCode:
				breakerPanel.FailWithTrip(cbKey, circuit.RateTripFunc(conf.ErrRate, conf.MinSample))
			case kiterrno.RPCTimeoutCode:
				breakerPanel.TimeoutWithTrip(cbKey, circuit.RateTripFunc(conf.ErrRate, conf.MinSample))
			default:
				breakerPanel.Succeed(cbKey)
			}
			return resp, err
		}
	}
}

// NewLoadbalanceMW .
func NewLoadbalanceMW(mwCtx context.Context) endpoint.Middleware {
	var lb loadbalancer.Loadbalancer
	opts := mwCtx.Value(KITC_OPTIONS_KEY).(*Options)

	if opts.Loadbalancer != nil {
		lb = opts.Loadbalancer
	} else {
		lb = loadbalancer.NewWeightLoadbalancer()
	}

	if rbl, ok := lb.(loadbalancer.Rebalancer); ok {
		handler := func(event *kitevent.KitEvent) {
			key := event.Extra["key"].(string)
			ins := event.Extra["new"].([]*discovery.Instance)

			if event.Name == SERVICE_DISCOVERY_SUCCESS && rbl.IsExist(key) {
				return
			}

			rbl.Rebalance(key, ins)
		}
		ebus := mwCtx.Value(KITC_EVENT_BUS_KEY).(kitevent.EventBus)
		ebus.Watch(SERVICE_DISCOVERY_SUCCESS, handler)
		ebus.Watch(SERVICE_ADDRESS_CHANGE, handler)
	}

	return func(next endpoint.EndPoint) endpoint.EndPoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			rpcInfo := GetRPCInfo(ctx)
			start := time.Now()
			insKey := rpcInfo.To + ":" + rpcInfo.TargetIDC() + ":" + rpcInfo.ToCluster + ":" + rpcInfo.Env

			if instances, ok := kitutil.GetCtxRPCInstances(ctx); ok && len(instances) > 0 {
				// Add special suffix to prevent loadbalancer from returning cached instances
				insKey += "(no cache)"
			}

			var lbRequest = ctx.Value(KITC_RAW_REQUEST_KEY)
			picker := lb.NewPicker(ctx, lbRequest, insKey, rpcInfo.Instances())

			var errs []error
			var resp interface{}
			var err error
			for {
				select {
				case <-ctx.Done():
					return nil, makeTimeoutErr(ctx, rpcInfo, start)
				default:
				}

				targetIns, ok := picker.Pick()
				if !ok {
					errs = append(errs, errors.New("No more instances to retry"))
					kerr := kiterrno.NewKiteError(kiterrno.KITE, kiterrno.ConnRetryCode, joinErrs(errs))
					return nil, kerr
				}

				rpcInfo.SetTargetInstance(targetIns)
				resp, err = next(ctx, request)
				if err == nil {
					return resp, err
				}
				errs = append(errs, newConnErr(targetIns, err))

				kerr, ok := err.(kiterrno.KiteError)
				if !ok || kerr.Category() != kiterrno.KITE {
					break
				}

				switch kerr.Errno() {
				case kiterrno.NotAllowedByInstanceCBCode:
					continue
				case kiterrno.GetConnErrorCode:
					logs.CtxWarn(ctx, "KITC: get conn for %v:%v:%v err: %v", rpcInfo.To, rpcInfo.ToCluster, rpcInfo.Method, err)
					continue
				}
				break
			}

			return resp, err
		}
	}
}

// OpenTracingMW creates a middleware for tracing functionality.
func OpenTracingMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		if _, ok := opentracing.GlobalTracer().(opentracing.NoopTracer); ok {
			return next(ctx, request)
		}
		r := GetRPCInfo(ctx)
		normOperation := trace.FormatOperationName(r.From, r.FromMethod)
		span, ctx := opentracing.StartSpanFromContext(ctx, normOperation)

		// pass trace context out of process using Base.Extra
		injectTraceIntoExtra(request, ctx)
		fillSpanDataBeforeCall(ctx, r)
		begin := time.Now()
		resp, err := next(ctx, request)
		finishOptions := fillSpanDataAfterCall(ctx, r, resp, err)

		fillPostSpanAfterCall(ctx, r, resp, err, begin)
		// finishing span
		span.FinishWithOptions(finishOptions)
		return resp, err
	}
}

func newConnErr(ins *discovery.Instance, err error) error {
	return fmt.Errorf("ins=%s err=%s", net.JoinHostPort(ins.Host, ins.Port), err)
}
