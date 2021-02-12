package mysql

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

var (
	ErrUnAssignedErrorCode = 100

	ErrForbiddenByDegradation     = errors.New("forbidden by degradation")
	ErrForbiddenByDegradationCode = 104 // see http://golang.byted.org/book/kite/_book/errorcode/errorcode.html

	ErrNotAllowedByServiceCB     = errors.New("not allowed by service cb")
	ErrNotAllowedByServiceCBCode = 101
)

var (
	serviceName    = "toutiao.unknown.unknown"
	serviceCluster = "default"
	unknown        = "unknown"
	localIP        = unknown
)

func init() {
	localIP = getLocalIP()

	if psm := os.Getenv("LOAD_SERVICE_NAME"); psm != "" {
		serviceName = psm
	} else if psm := os.Getenv("TCE_PSM"); psm != "" {
		serviceName = psm
	}

	if cluster := os.Getenv("SERVICE_CLUSTER"); cluster != "" {
		serviceCluster = cluster
	}
}

// SetPSMCluster
func SetPSMCluster(psm, cluster string) {
	serviceName = psm
	serviceCluster = cluster
}

// SetServiceAuthKey set the authkey to env
func SetServiceAuthKey(consulName, authKey string) {
	os.Setenv(consulName2EnvKey(consulName), authKey)
}

func getLocalIP() string {
	if localIP != "" {
		return localIP
	}

	ip := os.Getenv("HOST_IP_ADDR")
	ip = strings.TrimSpace(ip)

	if ip != "" {
		return ip
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return unknown
	}

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			ip4 := ip.To4()
			if ip4 == nil {
				continue
			}

			ip4Str := fmt.Sprintf("%v", ip4)
			if ip4Str == "127.0.0.1" {
				continue
			}
			return ip4Str
		}
	}
	return unknown
}

func getOperation(sql string) (string, int) {
	// skip leading space characters
	start := 0
	for start < len(sql) && isSpace(sql[start]) {
		start++
	}

	// find the first separating character
	pos := start
	for pos < len(sql) && isSpace(sql[pos]) == false {
		pos++
	}

	operation := strings.ToLower(sql[start:pos])
	return operation, pos
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

var quotes = []byte{'"', '\'', '`'}

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
		if interpolateFlag {
			lowerHint := strings.ToLower(interpolatedHint)
			idx = strings.Index(sql, lowerHint)
			if idx == -1 {
				return unknown
			}
			idx += len(lowerHint)
		} else {
			idx = strings.Index(sql, "update")
			if idx == -1 {
				return unknown
			}
			idx += len("update")
		}
	default:
		return ""
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

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func consulName2PSM(consulName string) string {
	if !strings.HasPrefix(consulName, "toutiao.mysql") {
		consulName = "toutiao.mysql." + consulName
	}
	return consulName
}

func getMysqlErrCode(err error) int {
	if err == nil {
		return 0
	}
	if err == driver.ErrSkip {
		return 0
	}

	if err == ErrForbiddenByDegradation {
		return ErrForbiddenByDegradationCode
	}

	if err == ErrNotAllowedByServiceCB {
		return ErrNotAllowedByServiceCBCode
	}

	if mysqlErr, ok := err.(*MySQLError); ok {
		return int(mysqlErr.Number)
	}
	return ErrUnAssignedErrorCode
}

func toutiaoSQLBefore(ctx context.Context, sql string, cfg *Config, mc *mysqlConn) error {
	r := getMysqlRPCMeta(ctx, cfg, sql)

	if doDegradationNew(r) {
		return ErrForbiddenByDegradation
	}

	if cbOpen(r) {
		return ErrNotAllowedByServiceCB
	}

	opentracingMW.ProcessRequest(ctx, &r, cfg, sql, mc)
	return nil
}

func toutiaoSQLAfter(ctx context.Context, sql string, cfg *Config, cost time.Duration, err error, mc *mysqlConn) {
	r := getMysqlRPCMeta(ctx, cfg, sql)
	opentracingMW.ProcessResponse(ctx, sql, cfg, err, mc, &r, cost)
	doMetrics(sql, cfg, cost, err)
	doSlowSQLLog(ctx, sql, cfg, cost, err)
	doCBMetrics(ctx, r, err)
}
