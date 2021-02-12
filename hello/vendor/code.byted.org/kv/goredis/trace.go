package goredis

import (
	"context"
	"fmt"
	"time"
	"unsafe"

	"code.byted.org/gopkg/env"
	kext "code.byted.org/kv/goredis/trace/ext"
	posttrace "code.byted.org/kv/goredis/trace/post-trace"
	"code.byted.org/kv/redis-v6"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	olog "github.com/opentracing/opentracing-go/log"
)

const (
	SNAMEKEY  = "K_SNAME"
	METHODKEY = "K_METHOD"
)

var noopSpan = opentracing.NoopTracer{}.StartSpan("noop")

func init() {
	UsePipeMiddleWare(&pipeOpenTracingExecMiddleWare{})
}

// -- opentracingMiddleWare Start --
type opentracingMiddleWare struct{}

func (m *opentracingMiddleWare) ProcessRequest(ctx *WrapProcessContext, cmd redis.Cmder) (redis.Cmder, error) {
	_, err := instrumentBeforeRequest(ctx.Client().ctx, ctx.Client(), cmd)
	return cmd, err
}

func (m *opentracingMiddleWare) ProcessResponse(ctx *WrapProcessContext, cmd redis.Cmder, err error) {
	instrumentAfterResponse(ctx.Client().ctx, ctx.Client(), cmd.Err(), cmd)
	postTraceAfterResponse(ctx.Client(), ctx.Client().ctx, cmd.Err(), cmd)
}

// -- opentracingMiddleWare End --

// -- pipeOpenTracingExecMiddleWare Start --
type pipeOpenTracingExecMiddleWare struct{}

func (m *pipeOpenTracingExecMiddleWare) ProcessRequest(pctx *PipeExecContext) error {
	_, err := instrumentBeforeRequest(
		pctx.Pipeline().Client().ctx,
		pctx.Pipeline().Client(),
		pctx.Pipeline().Cmds()...)

	return err
}

func (m *pipeOpenTracingExecMiddleWare) ProcessResponse(pctx *PipeExecContext, cmds []redis.Cmder, err error) {
	instrumentAfterResponse(pctx.Pipeline().Client().ctx, pctx.Pipeline().Client(), err, cmds...)
	postTraceAfterResponse(pctx.Pipeline().Client(), pctx.Pipeline().Client().ctx, err, cmds...)
}

// -- pipeExecMiddleWare End --

// -- following is instrument implement

func instrumentBeforeRequest(ctx context.Context,
	client *Client, cmders ...redis.Cmder) (reqLen int, err error) {
	if nil == client || len(cmders) == 0 {
		return 0, nil
	}

	// span instrument
	if ctx != nil {
		parentSpan := opentracing.SpanFromContext(ctx)
		if parentSpan != nil && isSampled(parentSpan) {
			/// start operation
			serviceName, operation := client.PSM(), "-"
			if psm, ok := getStrCtx(ctx, SNAMEKEY); ok {
				serviceName = psm
			}
			if method, ok := getStrCtx(ctx, METHODKEY); ok {
				operation = method
			}
			normOperation := serviceName + "::" + operation
			span, newCtx := opentracing.StartSpanFromContext(ctx, normOperation)
			client.ctx = newCtx

			ext.Component.Set(span, "goredis")
			ext.DBType.Set(span, "redis")
			ext.SpanKindRPCClient.Set(span)
			kext.LocalIDC.Set(span, env.IDC())
			kext.LocalCluster.Set(span, env.Cluster())

			command := cmders[0].Name()
			if len(cmders[0].Args()) > 1 {
				command = command + " " + fmt.Sprintf("%v", cmders[0].Args()[1])
			}
			if isPipeline := len(cmders) > 1; isPipeline {
				command = "pipeline: " + command + " ..."
			}
			ext.DBStatement.Set(span, command)
			for idx, cmd := range cmders {
				reqLen += fetchReqLength(cmd)
				if idx >= 1 {
					if len(cmd.Args()) > 1 {
						span.SetTag(fmt.Sprintf("Cmd%02d", idx),
							fmt.Sprintf("%v %v", cmd.Name(), cmd.Args()[1]))
					} else {
						span.SetTag(fmt.Sprintf("Cmd%02d", idx),
							fmt.Sprintf("%v ", cmd.Name()))
					}
				}
			}
			ext.PeerService.Set(span, client.MetricsServiceName()+"::"+cmders[0].Name())
			kext.RequestLength.Set(span, int32(reqLen))
		} else {
			client.ctx = opentracing.ContextWithSpan(ctx, noopSpan)
		}
	}

	return
}

func instrumentAfterResponse(ctx context.Context, client *Client,
	cmdErr error, cmders ...redis.Cmder) (rspLen int) {
	if ctx != nil {
		span := opentracing.SpanFromContext(ctx)
		if span != nil {
			defer span.Finish()
			if !isSampled(span) {
				return
			}
			span.LogFields(kext.EventKindPkgRecvEnd)
			if cmdErr == nil {
				for _, cmd := range cmders {
					//rspLen += len(cmd.String())
					rspLen += fetchRspLength(cmd)
				}
				kext.ResponseLength.Set(span, int32(rspLen))
			} else if cmdErr == redis.Nil {
				kext.ResponseLength.Set(span, 0)
			} else {
				span.LogFields(olog.String("error.kind", cmdErr.Error()))
			}
			ext.Error.Set(span, cmdErr != nil && cmdErr != redis.Nil)
		}
	}
	return
}

func postTraceAfterResponse(client *Client, ctx context.Context,
	cmdErr error, cmders ...redis.Cmder) {
	if ctx == nil || client == nil || len(cmders) == 0 {
		return
	}
	defer func() {
		client.ctx = client.Client.Context()
	}()

	if cmdErr == nil || cmdErr == redis.Nil {
		return
	}

	if isSampled(opentracing.SpanFromContext(ctx)) {
		return
	}

	recorder := posttrace.PostTraceRecorderFromContext(ctx)
	if recorder == nil {
		return
	}

	startopts := make([]opentracing.StartSpanOption, 0)
	startopts = append(startopts, opentracing.StartTime(time.Now()))
	kvtags := make(map[string]interface{})

	peerOperation := client.MetricsServiceName() + "::" + cmders[0].Name()
	kvtags[string(ext.Component)] = "goredis"
	kvtags[string(ext.DBType)] = "redis"
	kvtags[ext.SpanKindRPCClient.Key] = ext.SpanKindRPCClient.Value
	kvtags[string(ext.PeerService)] = peerOperation

	kvtags[string(ext.Error)] = true

	command := cmders[0].Name()
	if len(cmders[0].Args()) > 1 {
		command = command + " " + fmt.Sprintf("%v", cmders[0].Args()[1])
	}
	if isPipeline := len(cmders) > 1; isPipeline {
		command = "pipeline: " + command + " ..."
	}
	kvtags[string(ext.DBStatement)] = command
	var reqLen int
	for idx, cmd := range cmders {
		reqLen += fetchReqLength(cmd)
		if idx >= 1 {
			if len(cmd.Args()) > 1 {
				kvtags[fmt.Sprintf("Cmd%02d", idx)] =
					fmt.Sprintf("%v %v", cmd.Name(), cmd.Args()[1])
			} else {
				kvtags[fmt.Sprintf("Cmd%02d", idx)] =
					fmt.Sprintf("%v ", cmd.Name())
			}
		}
	}
	kvtags[string(kext.RequestLength)] = int32(reqLen)

	logField := make([]olog.Field, 0)
	logField = append(logField, olog.String("error.kind", cmdErr.Error()))
	var finishOpts opentracing.FinishOptions
	finishOpts.LogRecords = append(finishOpts.LogRecords, opentracing.LogRecord{
		Timestamp: time.Now(),
		Fields:    logField,
	})
	finishOpts.FinishTime = time.Now()
	startopts = append(startopts, opentracing.Tags(kvtags))
	recorder.RecordChild(peerOperation, startopts, finishOpts)
}

func fetchReqLength(cmder redis.Cmder) int {
	if cmder == nil {
		return 0
	}
	var reqLen int
	for _, arg := range cmder.Args() {
		switch v := arg.(type) {
		case string:
			reqLen += len(v)
		case []byte:
			reqLen += len(v)
		default:
		}
	}
	return reqLen
}

func fetchRspLength(cmder redis.Cmder) int {
	if cmder == nil {
		return 0
	}

	var rspLen int
	switch cmd := cmder.(type) {
	case *redis.BoolCmd:
		rspLen = int(unsafe.Sizeof(cmd.Val()))
	case *redis.BoolSliceCmd:
		if len(cmd.Val()) > 0 {
			rspLen = int(unsafe.Sizeof(cmd.Val()[0])) * len(cmd.Val())
		}
	case *redis.DurationCmd:
		rspLen = int(unsafe.Sizeof(cmd.Val()))
	case *redis.FloatCmd:
		rspLen = int(unsafe.Sizeof(cmd.Val()))
	case *redis.IntCmd:
		rspLen = int(unsafe.Sizeof(cmd.Val()))
	case *redis.ScanCmd:
		page, cursor := cmd.Val()
		for _, v := range page {
			rspLen += len(v)
		}
		rspLen += int(unsafe.Sizeof(cursor))
	case *redis.ScanRowCmd:
		if byteRst, ok := cmd.Val().([][]byte); ok {
			for _, v := range byteRst {
				rspLen += len(v)
			}
		}
	case *redis.SliceCmd:
		for _, rst := range cmd.Val() {
			if v, ok := rst.(string); ok {
				rspLen += len(v)
			}
		}
	case *redis.StatusCmd:
		rspLen += len(cmd.Val())
	case *redis.StringCmd:
		rspLen += len(cmd.Val())
	case *redis.StringSliceCmd:
		for _, rst := range cmd.Val() {
			rspLen += len(rst)
		}
	case *redis.TimeCmd:
		rspLen += int(unsafe.Sizeof(cmd.Val()))
	case *redis.ZSliceCmd:
		for _, rst := range cmd.Val() {
			if v, ok := rst.Member.(string); ok {
				rspLen += len(v)
			}
		}
		if len(cmd.Val()) > 0 {
			rspLen += len(cmd.Val()) * int(unsafe.Sizeof(cmd.Val()[0].Score))
		}
	//case *redis.XGetCmd:
	//case *redis.XSetCmd:
	default:
	}

	return rspLen
}

func isSampled(span opentracing.Span) bool {
	if span == nil {
		return false
	}
	type jaegerSpan interface {
		IsSampled() bool
	}
	if jSpan, ok := span.(jaegerSpan); ok {
		return jSpan.IsSampled()
	}
	return false
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
