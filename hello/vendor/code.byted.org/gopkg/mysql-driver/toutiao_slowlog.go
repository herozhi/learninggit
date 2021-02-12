package mysql

import (
	"context"
	"fmt"
	"time"
)

// TraceLogger .
type TraceLogger interface {
	Trace(format string, v ...interface{})
	Error(format string, v ...interface{})
}

const (
	// LOGIDKEY .
	LOGIDKEY = "K_LOGID"
)

var (
	slowSQLLogLogger    TraceLogger
	slowSQLLogFlag      bool
	slowSQLLogThreshold time.Duration
	// localIP from logID fromCluster
	slowSQLLogPatten = "%s %s %s %s rip=%s from_method=%s called=%s env=%s method=%s to_cluster=%s rpc_status=%s status=%d cost=%d sql=%s"
)

// OpenSlowSQLLog .
func OpenSlowSQLLog(threshold time.Duration, logger TraceLogger) error {
	if threshold <= 0 {
		return fmt.Errorf("slow sql log threshold must be larger than zero")
	}
	if logger == nil {
		return fmt.Errorf("nil trace logger")
	}

	slowSQLLogFlag = true
	slowSQLLogLogger = logger
	slowSQLLogThreshold = threshold
	return nil
}

func doSlowSQLLog(ctx context.Context, sql string, cfg *Config, cost time.Duration, err error) {
	if !slowSQLLogFlag {
		return
	}
	if cost < slowSQLLogThreshold {
		return
	}

	logid := "-"
	if ctx != nil {
		tmp := ctx.Value(LOGIDKEY)
		if tmp != nil {
			if str, ok := tmp.(string); ok {
				logid = str
			}
		}
	}

	rip := cfg.Addr
	to := consulName2PSM(cfg.toutiaoConsulName)
	toCluster := "default"
	costInUS := int64(cost / 1000)
	code := getMysqlErrCode(err)
	method, _ := getOperation(sql)
	rpcStatus := "success"
	if err != nil {
		rpcStatus = "failed"
	}
	var sqlPrefix string
	if len(sql) < 100 {
		sqlPrefix = sql
	} else {
		sqlPrefix = sql[:100]
	}

	log := fmt.Sprintf(slowSQLLogPatten, localIP, serviceName, logid, serviceCluster,
		rip, "-", to, "-", method, toCluster, rpcStatus, code, costInUS, sqlPrefix)

	if err != nil {
		slowSQLLogLogger.Error(log)
	} else {
		slowSQLLogLogger.Trace(log)
	}
}
