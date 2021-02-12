/*
@zhengjianbo: mesh相关中间件，非mesh功能不要添加
*/
package kitc

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"

	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/logs"
	"code.byted.org/gopkg/metainfo"
	"code.byted.org/gopkg/thrift"
	"code.byted.org/kite/endpoint"
	"code.byted.org/kite/kitc/connpool"
	"code.byted.org/kite/kitutil/kiterrno"
)

// TODO(zhanggongyuan): remember to add opentracing instrument to record conn-event
// TODO(zhengjianbo): mesh场景下只直连proxy，pool可定制化下
func NewMeshPoolMW(mwCtx context.Context) endpoint.Middleware {
	name := mwCtx.Value(KITC_CLIENT_KEY).(*KitcClient).Name()
	pool := connpool.NewLongPool(1000, 1<<20, 5*time.Second, name)
	handlePoolEvent(mwCtx, pool)

	return func(next endpoint.EndPoint) endpoint.EndPoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			rpcInfo := GetRPCInfo(ctx)
			begin := time.Now()

			ins := rpcInfo.TargetInstance()
			conn, err := pool.Get(
				ins.Network(),
				ins.Address(),
				time.Duration(defaultMeshProxyConfig.ConnectTimeout)*time.Millisecond)

			atomic.StoreInt32(&rpcInfo.ConnCost, int32(time.Now().Sub(begin)/time.Microsecond))
			if err != nil {
				kerr := kiterrno.NewKiteError(kiterrno.KITE, kiterrno.GetConnErrorCode, err)
				return nil, kerr
			}

			rpcInfo.SetConn(&connpool.ConnWithPkgSize{
				Conn: conn,
			})
			resp, err := next(ctx, request)

			if shouldDiscardConn(ctx, err) {
				pool.Discard(conn)
			} else {
				pool.Put(conn)
			}
			return resp, err
		}
	}
}

func shouldDiscardConn(ctx context.Context, err error) (should bool) {
	if err != nil {
		return true
	}
	if ctx == nil {
		return false
	}

	_, should = metainfo.GetBackwardValue(ctx, HeaderConnectionReadyToReset)
	if should {
		logs.Debug("discarding the connection because peer will shutdown later, which don't make any error.user could ignore it.")
	}
	return
}

// Mesh THeader set Headers .
func MeshSetHeadersMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		r := GetRPCInfo(ctx)
		headers := map[uint16]string{}
		headers[WITH_MESH_HEADER] = MESH_OR_TTHEADER_HEADER // response with mesh header(rip etc)
		headers[LOG_ID] = r.LogID                           // logid
		headers[ENV] = r.Env
		headers[FROM_SERVICE] = r.From        // from_service
		headers[FROM_CLUSTER] = r.FromCluster // from_cluster
		headers[FROM_IDC] = env.IDC()         // from_idc
		headers[FROM_METHOD] = r.FromMethod   // from_method
		headers[TO_SERVICE] = r.To            // to_service
		headers[TO_METHOD] = r.Method         // to_method
		// optional field
		if len(r.ToCluster) > 0 {
			headers[TO_CLUSTER] = r.ToCluster // to_cluster
		}
		if idc := r.TargetIDC(); len(idc) > 0 {
			headers[TO_IDC] = idc
		}
		// user define dest_address
		if inss := r.Instances(); len(inss) > 0 {
			var buf bytes.Buffer
			for i, ins := range inss {
				if i == 0 {
					buf.WriteString(net.JoinHostPort(ins.Host, ins.Port))
				} else {
					buf.WriteString("," + net.JoinHostPort(ins.Host, ins.Port))
				}
			}
			headers[DEST_ADDRESS] = buf.String()
		}
		// 用户指定超时配置rt, ct, rdt, wrt
		if r.RPCTimeout >= 0 {
			headers[RPC_TIMEOUT] = strconv.Itoa(r.RPCTimeout) // rpc_timeout
		}
		if r.ConnectTimeout >= 0 {
			headers[CONN_TIMEOUT] = strconv.Itoa(r.ConnectTimeout) // conn_timeout
		}
		if len(r.RingHashKey) > 0 {
			headers[RING_HASH_KEY] = r.RingHashKey
		}
		if spanCtxStr, exist := spanStringFromContext(ctx); exist {
			headers[SPAN_CTX] = spanCtxStr
		}
		if len(r.StressTag) > 0 {
			headers[STRESS_TAG] = r.StressTag // stress_tag
		}
		ctx = context.WithValue(ctx, THeaderInfoIntHeaders, headers)

		if metainfo.HasMetaInfo(ctx) {
			meta := make(map[string]string)
			metainfo.SaveMetaInfoToMap(ctx, meta)
			ctx = context.WithValue(ctx, THeaderInfoHeaders, meta)
		}

		return next(ctx, request)
	}
}

// TODO(zhanggongyuan): remember to add opentracing instrument to record send-recv pkg event
// MeshIOErrorHandlerMW .
func MeshIOErrorHandlerMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		resp, err := next(ctx, request)
		if err == nil {
			return resp, nil
		}
		if terr, ok := err.(thrift.TApplicationException); ok {
			errMsg := terr.Error()

			// restore the error information from kite server side
			kerr := kiterrno.ParseErrorMessage(errMsg)
			if ke, ok := kerr.(kiterrno.KiteError); ok {
				return nil, ke
			}

			// recognized as mesh proxy error
			typeId := int(terr.TypeId())
			if kiterrno.IsMeshErrorCode(typeId) {
				me := kiterrno.NewKiteError(kiterrno.MESH, typeId, terr)
				return nil, me
			}
		}
		// native thrift error will be wrapped as RemoteOrNetworkError
		return nil, kiterrno.NewKiteError(kiterrno.KITE, kiterrno.RemoteOrNetErrCode, err)
	}
}

// MeshRPCTimeoutMW . delete when buffered kill
func MeshRPCTimeoutMW(next endpoint.EndPoint) endpoint.EndPoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		rpcInfo := GetRPCInfo(ctx)
		start := time.Now()
		// plus 1ms wait for proxy response
		ctx, cancel := context.WithTimeout(ctx, time.Duration(rpcInfo.RPCTimeout)*time.Millisecond+meshMoreTimeout)
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

					logs.CtxError(ctx, "KITC: panic: to=%s, toCluster=%s, method=%s, Request: %v, err: %v\n%s",
						rpcInfo.To, rpcInfo.ToCluster, rpcInfo.Method, request, err, buf)

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
