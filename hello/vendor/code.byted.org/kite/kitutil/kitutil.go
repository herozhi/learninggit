package kitutil

import (
	"time"

	"code.byted.org/gopkg/logs"
	"code.byted.org/kite/endpoint"
)

// TraceFunc .
func TraceFunc(name string, start time.Time) {
	logs.Debug("Enter %s cost %s", name, time.Since(start))
}

type kiteBase struct {
	client string
	logID  string
	addr   string
	caller string
	extra  map[string]string
}

// ConvertRequestBase .
func ConvertRequestBase(base endpoint.ThriftBase) endpoint.KiteBase {
	if base == nil {
		return nil
	}
	return &kiteBase{
		client: base.GetClient(),
		logID:  base.GetLogID(),
		addr:   base.GetAddr(),
		caller: base.GetCaller(),
		extra:  base.GetExtra(),
	}
}

func (kb *kiteBase) GetClient() string {
	return kb.client
}

func (kb *kiteBase) GetAddr() string {
	return kb.addr
}

func (kb *kiteBase) GetCaller() string {
	return kb.caller
}

func (kb *kiteBase) GetLogID() string {
	return kb.logID
}

func (kb *kiteBase) GetEnv() string {
	return kb.extra["env"]
}

func (kb *kiteBase) GetCluster() string {
	return kb.extra["cluster"]
}

func (kb *kiteBase) GetExtra() map[string]string {
	return kb.extra
}
