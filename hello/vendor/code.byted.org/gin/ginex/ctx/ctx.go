// Package ctx(context) adds extra context informations in gin.Context, which will be used in log, metrics, and thrift rpc call
//
// Added context keys includes:
//   - log id (K_LOGID)
//   - local service name (K_SNAME)
//   - local ip (K_LOCALIP)
//   - local cluster (K_CLUSTER)
//   - method (K_METHOD)
package ctx

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.byted.org/gin/ginex/internal"
	internal_util "code.byted.org/gin/ginex/internal/util"
	"github.com/gin-gonic/gin"
)

var (
	localIP           string
	fullLengthLocalIP []byte
	isIPV4            bool
)

func Ctx() gin.HandlerFunc {
	psm := os.Getenv(internal.GINEX_PSM)
	cluster := internal_util.LocalCluster()
	return func(c *gin.Context) {
		c.Header(internal.HH_UPSTREAM_CAUGHT, strconv.FormatInt(time.Now().Round(0).UnixNano()/1e3, 10))

		// logID is set into response via request header if it exist, or by creating a new one
		logID := c.Request.Header.Get(internal.TT_LOGID_HEADER_KEY)
		if logID == "" {
			logID = c.Request.Header.Get(internal.TT_LOGID_HEADER_FALLBACK_KEY)
			if logID == "" {
				logID = genLogId()
			}
		}
		c.Set(internal.LOGIDKEY, logID)
		c.Header(internal.TT_LOGID_HEADER_KEY, logID)

		if env := c.Request.Header.Get(internal.TT_ENV_KEY); env != "" {
			c.Set(internal.ENVKEY, env)
		} else {
			c.Set(internal.ENVKEY, "prod")
		}
		if stressTag := c.Request.Header.Get(internal.TT_STRESS_KEY); stressTag != "" {
			c.Set(internal.STRESSKEY, stressTag)
		}
		if traceTag := c.Request.Header.Get(internal.TT_TRACE_TAG); traceTag != "" {
			c.Set(internal.TT_TRACE_TAG, traceTag)
		}
		c.Set(internal.SNAMEKEY, psm)
		c.Set(internal.LOCALIPKEY, localIP)
		c.Set(internal.CLUSTERKEY, cluster)
		method := internal.GetHandlerName(c.Handler())
		if method == "" {
			method = c.HandlerName()
			pos := strings.LastIndexByte(method, '.')
			if pos != -1 {
				method = c.HandlerName()[pos+1:]
			}
		}
		c.Set(internal.METHODKEY, method)
		for key := range c.Request.Header {
			if strings.HasPrefix(key, internal.RPC_PERSIST_PREFIX) {
				value := c.Request.Header.Get(key)
				c.Set(key, value)
			}
		}
		// 防止并发读写map
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), internal.CONTEXT_MUTEX, &sync.RWMutex{}))
	}
}

// genLogId generates a global unique log id for request
// IPV4 format: %Y%m%d%H%M%S + ip + 5位随机数
// IPV6 format: 版本号[2位 02] + 毫秒时间戳[13位] + IPv6[32位] + 6位随机数
// python runtime使用的random uuid, 这里简单使用random产生一个5位数字随机串
func genLogId() string {
	buf := make([]byte, 0, 64)

	if isIPV4 {
		buf = time.Now().AppendFormat(buf, "20060102150405")
	} else {
		ts := strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10)
		buf = append(buf, []byte("02")...)
		buf = append(buf, []byte(ts)...)
	}

	buf = append(buf, fullLengthLocalIP...)

	uuidBuf := make([]byte, 4)
	_, err := rand.Read(uuidBuf)
	if err != nil {
		panic(err)
	}
	uuidNum := binary.BigEndian.Uint32(uuidBuf)
	if isIPV4 {
		buf = append(buf, fmt.Sprintf("%05d", uuidNum)[:5]...)
	} else {
		buf = append(buf, fmt.Sprintf("%06d", uuidNum)[:6]...)
	}
	return string(buf)
}

func init() {
	// try to use IPV4 address first
	// use IPV6 address on IPV6-only mode
	localIP = os.Getenv(internal.MY_HOST_IP)
	if localIP == "" {
		localIP = os.Getenv(internal.MY_HOST_IPV6)
	}
	if localIP == "" {
		localIP = internal_util.LocalIP()
	}

	var elements []string
	if strings.Contains(localIP, ".") {
		isIPV4 = true
		elements = strings.Split(localIP, ".")
		for i := 0; i < len(elements); i++ {
			elements[i] = fmt.Sprintf("%03s", elements[i])
		}
	} else if strings.Contains(localIP, ":") {
		isIPV4 = false
		elements = strings.Split(localIP, ":")
		for i := 0; i < len(elements); i++ {
			elements[i] = fmt.Sprintf("%04s", elements[i])
		}
	} else {
		// use 127.000.000.001 as default
		isIPV4 = true
		elements = []string{"127", "000", "000", "001"}
	}
	fullLengthLocalIP = []byte(strings.Join(elements, ""))
}
