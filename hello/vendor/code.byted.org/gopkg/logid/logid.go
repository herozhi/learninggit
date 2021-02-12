package logid

import (
	"strconv"
	"strings"

	"code.byted.org/gopkg/ctxvalues"
	"code.byted.org/gopkg/net2"
	"code.byted.org/gopkg/rand"
)

const (
	// 目前版本为 02
	version    = "02"
	length     = 53
	maxRandNum = 1<<24 - 1<<20
)

// LogID represents a logID generator
type LogID struct {
	rand rand.Rand
}

// NewLogID create a new LogID instance
func NewLogID() LogID {
	return LogID{rand: rand.NewRand()}
}

// GenLogID return a new logID string
func (l LogID) GenLogID() string {
	ip := formatIP(net2.GetLocalIP())
	r := l.rand.Intn(maxRandNum) + 1<<20
	sb := strings.Builder{}
	sb.Grow(length)
	sb.WriteString(version)
	sb.WriteString(strconv.FormatInt(getMSTimestamp(), 10))
	sb.Write(ip)
	sb.WriteString(strconv.FormatInt(int64(r), 16))
	return sb.String()
}

var defaultLogID LogID

func init() {
	defaultLogID = NewLogID()
}

// GenLogID return a new logID
func GenLogID() string {
	return defaultLogID.GenLogID()
}

// CtxLogIDKey ByteDance Ctx Key of LogID
var CtxLogIDKey = ctxvalues.CTXKeyLogID

// GetLogIDFromCtx return logID from context
var GetLogIDFromCtx = ctxvalues.LogID
