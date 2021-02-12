package bytedmysql

import (
	"fmt"
	"os"
	"strings"

	"code.byted.org/gopkg/env"
)

const (
	unknownIP            = "unknown"
	maxAllowedPacketSize = 4 << 20 // 4 MiB 待确定是跟官方库还是我们自定义
)

var (
	interpolatedHint string
	interpolatedStmt = map[string]bool{
		// Data Manipulation Statements
		"delete":  true,
		"insert":  true,
		"select":  true,
		"update":  true,
		"replace": true,
	}
)

// 过滤 MySQL 的关键字
func validate(str string) string {
	str = strings.TrimSpace(str)
	str = strings.ToLower(str)
	str = strings.Replace(str, "*", "#", -1)
	str = strings.Replace(str, "delete", "de##te", -1)
	str = strings.Replace(str, "drop", "dr#p", -1)
	str = strings.Replace(str, "update", "up##te", -1)
	return str
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
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

func init() {
	serviceName := env.PSM()
	if serviceName == env.PSMUnknown {
		serviceName = "toutiao.known.known"
	}
	if len(serviceName) > 200 {
		serviceName = serviceName[:200]
	}
	serviceName = validate(serviceName)

	ip := env.HostIP()
	ip = validate(ip)

	interpolatedHint = fmt.Sprintf(" /* psm=%v, ip=%v, pid=%v */ ", serviceName, ip, os.Getpid())
}

// 将 PSM，IP，pid 等信息注入 SQL 中，如果注入出错，则返回原 SQL
func interpolate(sql string) string {
	if len(sql)+len(interpolatedHint) > maxAllowedPacketSize {
		return sql
	}

	operation, pos := getOperation(sql)
	if _, ok := interpolatedStmt[operation]; ok {
		sql = sql[:pos] + interpolatedHint + sql[pos:]
	}
	return sql
}
