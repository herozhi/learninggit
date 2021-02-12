package mysql

import (
	"fmt"
	"strconv"
	"time"

	"database/sql/driver"

	"code.byted.org/gopkg/metrics"
)

var (
	metricsCli     = metrics.NewDefaultMetricsClient("toutiao.service.thrift", true)
	metricsAuthCli = metrics.NewDefaultMetricsClient("toutiao.mysql.dbauth", true)

	successRPCThroughputFmt = "%s.call.success.throughput"
	errorRPCThroughputFmt   = "%s.call.error.throughput"
	successRPCLatencyFmt    = "%s.call.success.latency.us"
	errorRPCLatencyFmt      = "%s.call.error.latency.us"
	AuthReqThroughputFmt    = "client.throughput"
	AuthReqLatencyFmt       = "client.latency"
)

func doMetrics(sql string, cfg *Config, cost time.Duration, err error) {
	if err == driver.ErrSkip {
		return
	}
	operation, _ := getOperation(sql)

	to := consulName2PSM(cfg.toutiaoConsulName)
	costInUS := int64(cost / 1000)
	tags := map[string]string{
		"to":           to,
		"method":       operation,
		"mode":         Mode,
		"from_cluster": serviceCluster,
		"to_cluster":   "default",
		"table":        "unknow",
		"toHost":       cfg.Addr,
	}
	if tabel := getTableName(operation, sql); len(tabel) != 0 {
		tags["table"] = tabel
	}

	var throughputMetrics, latencyMetrics string
	errCode := getMysqlErrCode(err)
	if errCode == 0 {
		throughputMetrics = fmt.Sprintf(successRPCThroughputFmt, serviceName)
		latencyMetrics = fmt.Sprintf(successRPCLatencyFmt, serviceName)
	} else {
		throughputMetrics = fmt.Sprintf(errorRPCThroughputFmt, serviceName)
		latencyMetrics = fmt.Sprintf(errorRPCLatencyFmt, serviceName)
		tags["err_code"] = strconv.Itoa(errCode)
	}

	metricsCli.EmitCounter(throughputMetrics, 1, "", tags)
	metricsCli.EmitTimer(latencyMetrics, costInUS, "", tags)
}

type Metrics_Info struct {
	ServiceName string
	Psm         string
	Cost        int64  //请求耗时
	ErrCode     int    //0 正确   1 服务错误造成鉴权失败   2 后端服务问题
	Host        string // 后端服务地址
}

func doAuthMetrics(info *Metrics_Info) {

	costInUS := int64(info.Cost)
	tags := map[string]string{
		"psm":         info.Psm,
		"serviceName": info.ServiceName,
		"addr":        info.Host,
		"err_code":    fmt.Sprintf("%d", info.ErrCode),
	}

	var throughputMetrics, latencyMetrics string
	throughputMetrics = fmt.Sprintf(AuthReqThroughputFmt)
	latencyMetrics = fmt.Sprintf(AuthReqLatencyFmt)
	if err := metricsAuthCli.EmitCounter(throughputMetrics, 1, "", tags); err != nil {
		fmt.Println("err :", err.Error())
	}
	if err := metricsAuthCli.EmitTimer(latencyMetrics, costInUS, "", tags); err != nil {
		fmt.Println("err :", err.Error())
	}
}
