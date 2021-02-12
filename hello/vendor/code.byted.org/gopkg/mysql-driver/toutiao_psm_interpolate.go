package mysql

import (
	"fmt"
	"os"
	"strings"

	"code.byted.org/gopkg/env"
)

var (
	maxAllowedPacketSize = 512 * (1 << 20) // 512 MB
	interpolatedHint     string
	interpolateFlag      = false

	interpolatedStmt = map[string]bool{
		// Data Manipulation Statements
		"delete":  true,
		"insert":  true,
		"select":  true,
		"update":  true,
		"replace": true,
	}
)

func init() {
	if (env.PSM() != "") {
		OpenInterpolation(env.PSM())
	}
}

// validate replaces some dangerous key words
func validate(str string) string {
	str = strings.TrimSpace(str)
	str = strings.ToLower(str)
	str = strings.Replace(str, "*", "#", -1)
	str = strings.Replace(str, "delete", "de##te", -1)
	str = strings.Replace(str, "drop", "dr#p", -1)
	str = strings.Replace(str, "update", "up##te", -1)
	return str
}

// OpenInterpolation x
func OpenInterpolation(psm string) {
	if len(psm) > 200 {
		psm = psm[:200]
	}
	psm = validate(psm)

	ip := getLocalIP()
	ip = validate(ip)

	interpolateFlag = true
	interpolatedHint = fmt.Sprintf(" /* psm=%v, ip=%v, pid=%v */ ", psm, ip, os.Getpid())
}

// interpolatePSM interpolates PSM to this SQL;
// If failed, return the original SQL without any change.
func interpolatePSM(sql string) string {
	if interpolateFlag == false {
		return sql
	}

	if len(sql)+len(interpolatedHint) > maxAllowedPacketSize {
		return sql
	}

	operation, pos := getOperation(sql)
	if _, ok := interpolatedStmt[operation]; ok {
		sql = sql[:pos] + interpolatedHint + sql[pos:]
	}
	return sql
}
