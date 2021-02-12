// Package accesslog implements a middleware that emit an access log for each request.
//
// Includes fields like:
//   - status
//   - method
//   - client ip
//   - latency
package accesslog

import (
	"context"
	"os"
	"time"

	"code.byted.org/gin/ginex/internal"
	internal_util "code.byted.org/gin/ginex/internal/util"
	"code.byted.org/gopkg/logs"
	"github.com/gin-gonic/gin"
)

// CtxTraceKVLogger .
type CtxTraceKVLogger interface {
	CtxTraceKVs(ctx context.Context, kvs ...interface{})
	CtxErrorKVs(ctx context.Context, kvs ...interface{})
}

func AccessLog(logger *logs.Logger) gin.HandlerFunc {
	psm := os.Getenv(internal.GINEX_PSM)
	cluster := internal_util.LocalCluster()

	var kvLogger CtxTraceKVLogger
	var i interface{} = logger
	if kvl, ok := i.(CtxTraceKVLogger); ok {
		kvLogger = kvl
	}

	return func(c *gin.Context) {
		if logger == nil {
			// 不能初始化log不是critical error, 并不影响正常运行
			c.Next()
			return
		}

		// some evil middlewares modify this values
		path := c.Request.URL.Path
		start := time.Now()
		c.Next()
		end := time.Now()
		latency := end.Sub(start).Nanoseconds() / 1000
		localIp := c.Value(internal.LOCALIPKEY)
		stressTag := c.Value(internal.STRESSKEY)
		var handleMethod string
		if v := c.Value(internal.METHODKEY); v != nil {
			handleMethod = v.(string)
		}
		if stressTag == nil {
			if kvLogger == nil {
				// status, latency, method, path, remote_ip, psm, log_id, local_cluster, host, user_agent
				logger.Trace("%s %s %s %s status=%d cost=%d method=%s handle_method=%s full_path=%s client_ip=%s host=%s",
					localIp, psm, c.Value(internal.LOGIDKEY), cluster, c.Writer.Status(), latency,
					c.Request.Method, handleMethod, path, c.ClientIP(), c.Request.Host)
			} else {
				kvs := []interface{}{"status", c.Writer.Status(),
					"cost", latency,
					"method", c.Request.Method,
					"handle_method", handleMethod,
					"full_path", path,
					"client_ip", c.ClientIP(),
					"host", c.Request.Host}
				ctx := context.WithValue(context.Background(), internal.LOGIDKEY, c.Value(internal.LOGIDKEY))
				kvLogger.CtxTraceKVs(ctx, kvs...)
			}
		} else {
			if kvLogger == nil {
				logger.Trace("%s %s %s %s status=%d cost=%d method=%s handle_method=%s full_path=%s client_ip=%s host=%s stress_tag=%s",
					localIp, psm, c.Value(internal.LOGIDKEY), cluster, c.Writer.Status(), latency, c.Request.Method,
					handleMethod, path, c.ClientIP(), c.Request.Host, stressTag)
			} else {
				kvs := []interface{}{"status", c.Writer.Status(),
					"cost", latency,
					"method", c.Request.Method,
					"handle_method", handleMethod,
					"full_path", path,
					"client_ip", c.ClientIP(),
					"host", c.Request.Host,
					"stress_tag", stressTag}
				ctx := context.WithValue(context.Background(), internal.LOGIDKEY, c.Value(internal.LOGIDKEY))
				kvLogger.CtxTraceKVs(ctx, kvs...)
			}
		}
	}
}
