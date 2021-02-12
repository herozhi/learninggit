package connpool

import (
	"net"

	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/metrics"
)

var (
	mclient *metrics.MetricsClient
)

func init() {
	psm := env.PSM()
	if psm == env.PSMUnknown {
		psm = "toutiao.unknown.unknown"
	}
	prefix := "go." + psm

	mclient = metrics.NewDefaultMetricsClient(prefix, true)
}

type ConnMetrics interface {
	ConnSucc(addr net.Addr)
	ConnFail(addr net.Addr)
	ReuseSucc(addr net.Addr)
}

type ShortConnMetrics string

// 受限于基础组件, 当下游addr太多时, opentsdb聚合会很慢甚至聚合不出来, 故暂时不把addr放到tag内
func (scm ShortConnMetrics) ConnSucc(addr net.Addr) {
	mclient.EmitCounter("to."+string(scm)+".get_short_conn_succ", 1, "", nil)
}

func (scm ShortConnMetrics) ConnFail(addr net.Addr) {
	mclient.EmitCounter("to."+string(scm)+".get_short_conn_fail", 1, "", nil)
}

func (scm ShortConnMetrics) ReuseSucc(addr net.Addr) {
}

type LongConnMetrics string

func (lcm LongConnMetrics) ConnSucc(addr net.Addr) {
	mclient.EmitCounter("to."+string(lcm)+".new_long_conn_succ", 1, "", nil)
}

func (lcm LongConnMetrics) ConnFail(addr net.Addr) {
	mclient.EmitCounter("to."+string(lcm)+".new_long_conn_fail", 1, "", nil)
}

func (lcm LongConnMetrics) ReuseSucc(addr net.Addr) {
	mclient.EmitCounter("to."+string(lcm)+".get_alive_long_conn_succ", 1, "", nil)
}
