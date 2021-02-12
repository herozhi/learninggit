package kite

import (
	"context"
	"time"

	"code.byted.org/gopkg/thrift"
	"code.byted.org/kite/kitc/connpool"
)

func ifelse(ok bool, onTrue, onFalse string) string {
	if ok {
		return onTrue
	} else {
		return onFalse
	}
}

func empty2Default(str, dft string) string {
	return ifelse(str == "", dft, str)
}

type statProtocol struct {
	thrift.TProtocol
	readBegin  time.Time // After ReadMessageBegin done
	readEnd    time.Time // After ReadMessageEnd done
	writeBegin time.Time // Before WriteMessageBegin
	writeEnd   time.Time // After Flush
}

func (sp *statProtocol) ReadMessageBegin() (name string, typeId thrift.TMessageType, seqid int32, err error) {
	name, typeId, seqid, err = sp.TProtocol.ReadMessageBegin()
	sp.readBegin = time.Now()
	return
}
func (sp *statProtocol) ReadMessageEnd() (err error) {
	err = sp.TProtocol.ReadMessageEnd()
	sp.readEnd = time.Now()
	return
}
func (sp *statProtocol) WriteMessageBegin(name string, typeId thrift.TMessageType, seqid int32) (err error) {
	sp.writeBegin = time.Now()
	err = sp.TProtocol.WriteMessageBegin(name, typeId, seqid)
	return
}
func (sp *statProtocol) Flush() (err error) {
	err = sp.TProtocol.Flush()
	sp.writeEnd = time.Now()
	return
}

type rpcStatsKey struct{}

func getRPCStats(ctx context.Context) (ss *rpcStats, ok bool) {
	ss, ok = ctx.Value(rpcStatsKey{}).(*rpcStats)
	return
}

func newCtxWithRPCStats(ctx context.Context, stat *rpcStats) context.Context {
	return context.WithValue(ctx, rpcStatsKey{}, stat)
}

type rpcStats struct {
	conn      *connpool.ConnWithPkgSize
	protocol  *statProtocol
	callbacks []func(*rpcStats)
}

func (rs *rpcStats) Reset() {
	rs.conn.Written = 0
	rs.conn.Readn = 0
	rs.callbacks = rs.callbacks[:0]
}

func (rs *rpcStats) AddCallback(cb func(*rpcStats)) {
	rs.callbacks = append(rs.callbacks, cb)
}

func (rs *rpcStats) preRequest() {}

func (rs *rpcStats) postRequest() {
	if rs == nil {
		return
	}
	for i := range rs.callbacks {
		rs.callbacks[i](rs)
	}
	rs.Reset()
}

func (rs *rpcStats) ReadSize() int32 {
	return rs.conn.Readn
}

func (rs *rpcStats) WriteSize() int32 {
	return rs.conn.Written
}

func (rs *rpcStats) ReadCostUS() int64 {
	return rs.protocol.readEnd.Sub(rs.protocol.readBegin).Microseconds()
}

func (rs *rpcStats) WriteCostUS() int64 {
	return rs.protocol.writeEnd.Sub(rs.protocol.writeBegin).Microseconds()
}

func (rs *rpcStats) ProcessCostUS() int64 {
	return rs.protocol.writeBegin.Sub(rs.protocol.readEnd).Microseconds()
}

func (rs *rpcStats) TotalCostUS() int64 {
	return rs.protocol.writeEnd.Sub(rs.protocol.readBegin).Microseconds()
}
