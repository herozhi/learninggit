package mysql

import (
	"net"
	"strconv"
	"time"
	"context"
	"database/sql/driver"

	"code.byted.org/gopkg/env"
	kext "code.byted.org/gopkg/mysql-driver/trace/ext"
	posttrace "code.byted.org/gopkg/mysql-driver/trace/post-trace"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	olog "github.com/opentracing/opentracing-go/log"
)

const METHODKEY = "K_METHOD"

var opentracingMW opentracingMiddleWare

// -- opentracingMiddleWare Start --
type opentracingMiddleWare struct{}

func (m *opentracingMiddleWare) ProcessRequest(ctx context.Context, r *MysqlReqMeta, cfg *Config, sql string, mc *mysqlConn) {
	instrumentBeforeRequest(ctx, r, cfg, sql, mc)
}

func (m *opentracingMiddleWare) ProcessResponse(ctx context.Context, sql string, cfg *Config, err error, mc *mysqlConn, r *MysqlReqMeta, cost time.Duration) {
	instrumentAfterResponse(ctx, err, mc)
	postTraceAfterResponse(ctx, err, mc, r, cfg, sql, cost)
}

func (m *opentracingMiddleWare) ProcessFinish(mc *mysqlConn, conn *ConnWithPkgSize) {
	if mc.span != nil {
		kext.RequestLength.Set(*mc.span, conn.Written)
		kext.ResponseLength.Set(*mc.span, conn.Readn)
		(*mc.span).Finish()
		mc.span = nil
	}
}

// -- opentracingMiddleWare End --

// -- following is instrument implement
func instrumentBeforeRequest(ctx context.Context, r *MysqlReqMeta, cfg *Config, sql string, mc *mysqlConn) {
	if len(sql) == 0 || mc == nil || ctx == nil || r == nil {
		return
	}

	// check trace enable
	if _, isNoopTracer := opentracing.GlobalTracer().(opentracing.NoopTracer); isNoopTracer {
		return
	}

	// check if parent span sampled
	parentSpan := opentracing.SpanFromContext(ctx)
	if !isSampled(parentSpan) {
		return
	}

	// start operation
	serviceName, operation := r.From, "-"
	if method, ok := ctx.Value(METHODKEY).(string); ok && len(method) != 0 {
		operation = method
	}
	normOperation := serviceName + "::" + operation
	span, _ := opentracing.StartSpanFromContext(ctx, normOperation)
	mc.span = &span
	ext.Component.Set(span, "gomysql")
	ext.DBType.Set(span, "mysql")
	ext.DBInstance.Set(span, cfg.DBName)
	ext.SpanKindRPCClient.Set(span)
	ext.DBUser.Set(span, cfg.User)
	// addr info
	if host, port, err := net.SplitHostPort(cfg.Addr); err == nil {
		ext.PeerHostIPv4.Set(span, InetAtoN(host))
		if portNum, err := strconv.Atoi(port); err == nil {
			ext.PeerPort.Set(span, uint16(portNum))
		}
	}
	kext.LocalIDC.Set(span, env.IDC())
	kext.LocalCluster.Set(span, env.Cluster())
	kext.LocalAddress.Set(span, mc.netConn.LocalAddr().String())

	ext.DBStatement.Set(span, sql)
	ext.PeerService.Set(span, r.To+"::"+r.Method)

	span.LogFields(kext.EventKindPkgSendStart)
	return
}

func instrumentAfterResponse(ctx context.Context, sqlErr error, mc *mysqlConn) {
	_, isNoopTracer := opentracing.GlobalTracer().(opentracing.NoopTracer)
	if isNoopTracer || ctx == nil || mc == nil || mc.span == nil {
		return
	}
	span := *mc.span

	if sqlErr != driver.ErrBadConn {
		span.LogFields(kext.EventKindPkgRecvEnd)
	}

	ext.Error.Set(span, sqlErr != nil)
	// err code
	if sqlErr != nil {
		kext.ReturnCode.Set(span, int32(getMysqlErrCode(sqlErr)))
		span.LogFields(olog.String("error.kind", sqlErr.Error()))
		mc.clearPkgSize()
	}
}

func postTraceAfterResponse(ctx context.Context, sqlErr error, mc *mysqlConn, r *MysqlReqMeta, cfg *Config, sql string, cost time.Duration) {
	if sqlErr == nil || isPostTraceIgnoredErrno(sqlErr) || ctx == nil || r == nil || mc == nil {
		return
	}
	if mc.span != nil && isSampled(*mc.span) {
		return
	}
	recorder := posttrace.PostTraceRecorderFromContext(ctx)
	if recorder == nil {
		return
	}

	startopts := make([]opentracing.StartSpanOption, 0)

	var finishOpts opentracing.FinishOptions
	finishOpts.FinishTime = time.Now()
	startopts = append(startopts, opentracing.StartTime(finishOpts.FinishTime.Add(-cost)))
	kvtags := make(map[string]interface{})

	peerOperation := r.To + "::" + r.Method
	kvtags[string(ext.Component)] = "gomysql"
	kvtags[string(ext.DBType)] = "mysql"
	kvtags[ext.SpanKindRPCClient.Key] = ext.SpanKindRPCClient.Value
	kvtags[string(ext.PeerService)] = peerOperation

	kvtags[string(ext.Error)] = true

	kvtags[string(ext.DBInstance)] = cfg.DBName
	kvtags[string(ext.DBUser)] = cfg.User

	// addr info
	if host, port, err := net.SplitHostPort(cfg.Addr); err == nil {
		kvtags[string(ext.PeerHostIPv4)] = InetAtoN(host)
		if portNum, err := strconv.Atoi(port); err == nil {
			kvtags[string(ext.PeerPort)] = uint16(portNum)
		}
	}
	kvtags[string(kext.LocalAddress)] = mc.netConn.LocalAddr().String()
	kvtags[string(ext.DBStatement)] = sql
	kvtags[string(kext.ReturnCode)] = int32(getMysqlErrCode(sqlErr))
	if conn, ok := mc.netConn.(*ConnWithPkgSize); ok && conn != nil {
		kvtags[string(kext.RequestLength)] = conn.Written
		kvtags[string(kext.ResponseLength)] = conn.Readn
	}
	logField := make([]olog.Field, 0)
	logField = append(logField, olog.String("error.kind", sqlErr.Error()))
	finishOpts.LogRecords = append(finishOpts.LogRecords, opentracing.LogRecord{
		Timestamp: time.Now(),
		Fields:    logField,
	})
	startopts = append(startopts, opentracing.Tags(kvtags))
	recorder.RecordChild(peerOperation, startopts, finishOpts)
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

func isPostTraceIgnoredErrno(err error) bool {
	switch err {
	case ErrForbiddenByDegradation,
		 ErrNotAllowedByServiceCB:
		return true
	default:
		return false
	}
}

func eventCollect(spanPtr *opentracing.Span, method string) (deferFunc func(error)) {
	if spanPtr == nil {
		return nil
	}
	span := *spanPtr
	span.LogKV("event", "perfT." + method + "Start")
	deferFunc = func(err error) {
		if err != nil {
			span.LogKV("error.kind", method + " error " + err.Error())
		} else {
			span.LogKV("event", "perfT." + method + "End")
		}
	}
	return
}