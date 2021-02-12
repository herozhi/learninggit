package throttle

import (
	"net/http"

	"code.byted.org/gin/ginex/apimetrics"
	"github.com/gin-gonic/gin"
)

const (
	DEFAULT_QPS_LIMIT = 50000
	DEFAULT_MAX_CON   = 10000
)

func Throttle(psm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !globalLimiter.TakeCon() {
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}
		// 只在有新增连接时打点，会丢失部分精度，但是能减少一半的metrics调用
		apimetrics.EmitCurrentConnectionCount(psm, globalLimiter.ConnNow())
		defer func() {
			e := recover()
			globalLimiter.ReleaseCon()
			if e != nil {
				panic(e)
			}
		}()

		if !globalLimiter.TakeQPS() {
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}
		c.Next()
	}
}
