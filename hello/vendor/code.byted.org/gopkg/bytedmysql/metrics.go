package bytedmysql

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"

	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/metrics"
)

// 不能识别的 errorCode, copy 以前的代码
const (
	unknown                = "unknown"
	errUnAssignedErrorCode = 100
)

var (
	metricsClient *metrics.MetricsClientV2

	successKey        string
	errorKey          string
	successLatencyKey string
	errorLatencyKey   string
)

var quotes = []byte{'"', '\'', '`'}

func init() {
	metricsClient = metrics.NewDefaultMetricsClientV2("", true)

	serviceName := env.PSM()
	if serviceName == env.PSMUnknown {
		serviceName = "toutiao.unknown.unknown"
	}

	successKey = fmt.Sprintf("toutiao.service.thrift.%s.call.success.throughput", serviceName)
	errorKey = fmt.Sprintf("toutiao.service.thrift.%s.call.error.throughput", serviceName)
	successLatencyKey = fmt.Sprintf("toutiao.service.thrift.%s.call.success.latency.us", serviceName)
	errorLatencyKey = fmt.Sprintf("toutiao.service.thrift.%s.call.error.latency.us", serviceName)
}

func getErrorCode(err error) uint16 {
	if err == nil {
		return 0
	}

	if err == driver.ErrSkip {
		return 0
	}

	if mysqlError, ok := err.(*mysql.MySQLError); ok {
		return mysqlError.Number
	}
	return errUnAssignedErrorCode
}

// 获取表名，拷贝之前的代码
func getTableName(op, sql string) string {
	op = strings.ToLower(op)
	sql = strings.ToLower(sql)
	var idx int
	switch op {
	case "insert":
		idx = strings.Index(sql, "into")
		if idx == -1 {
			return unknown
		}
		idx += len("into")
	case "select", "delete":
		idx = strings.Index(sql, "from")
		if idx == -1 {
			return unknown
		}
		idx += len("from")
	case "update":
		lowerHint := strings.ToLower(interpolatedHint)
		idx = strings.Index(sql, lowerHint)
		if idx == -1 {
			return unknown
		}
		idx += len(lowerHint)

	default:
		return unknown
	}

	table := getNextWord(sql, idx)
	if len(table) < 2 {
		return table
	}
	for _, q := range quotes {
		if table[0] == q && table[len(table)-1] == q {
			return table[1 : len(table)-1]
		}
	}
	return table
}

func getNextWord(str string, begin int) string {
	for begin < len(str) && isSpace(str[begin]) { // filter leading space
		begin++
	}
	left := begin
	for begin < len(str) && !isSpace(str[begin]) {
		begin++
	}
	right := begin
	return str[left:right]
}

func doMetrics(method, sql string, to string, cost int64, err error) {
	errorCode := getErrorCode(err)

	op, _ := getOperation(sql)
	tableName := getTableName(op, sql)

	tags := []metrics.T{
		{Name: "to", Value: to},
		{Name: "method", Value: method},
		{Name: "from_cluster", Value: env.Cluster()},
		{Name: "to_cluster", Value: "default"},
		{Name: "errCode", Value: strconv.Itoa(int(errorCode))},
		{Name: "language", Value: "go"},
		{Name: "ver", Value: Version},
		{Name: "table", Value: tableName},
	}

	if errorCode == 0 {
		_ = metricsClient.EmitCounter(successKey, 1, tags...)
		_ = metricsClient.EmitTimer(successLatencyKey, cost, tags...)
	} else {
		_ = metricsClient.EmitCounter(errorKey, 1, tags...)
		_ = metricsClient.EmitTimer(errorLatencyKey, cost, tags...)
	}
}

// auth metrics 不打了，把 psm 都打出来, metrics 会花的
