package apimetrics

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"code.byted.org/gin/ginex/internal"
	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/metrics"
	"github.com/gin-gonic/gin"
)

const (
	METRICS_PREFIX           = "toutiao.service.http"
	FRAMEWORK_METRICS_PREFIX = "toutiao.service"
)

var (
	emitter = metrics.NewDefaultMetricsClient(METRICS_PREFIX, true)
)

// SuccessDecider is a function used to decide the success or failed status and change tags if needed
type SuccessDecider func(c *gin.Context, tags map[string]string) (success bool, newTags map[string]string)

// MetricsWithSuccessDecider returns a middleware function that emits success metric (latency, throughput) if
// `successDecider` returns `true` for response status code, and emits an error otherwise.
func MetricsWithSuccessDecider(psm string, meshMode bool, successDecider SuccessDecider) gin.HandlerFunc {
	succLatencyMetrics := fmt.Sprintf("%s.calledby.success.latency.us", psm)
	succThroughputMetrics := fmt.Sprintf("%s.calledby.success.throughput", psm)
	errLatencyMetrics := fmt.Sprintf("%s.calledby.error.latency.us", psm)
	errThroughputMetrics := fmt.Sprintf("%s.calledby.error.throughput", psm)
	frameWorkThroughputMetrics := "ginex.throughput"
	frameWorkMetricsTags := map[string]string{
		"version": internal.VERSION,
	}
	return func(c *gin.Context) {
		if psm == "" {
			c.Next()
			return
		}
		defer func() {
			if e := recover(); e != nil {
				// Check for a broken connection, as it is not really a panic
				var brokenPipe bool
				if ne, ok := e.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") || strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}
				if !brokenPipe {
					EmitPanicCounter(psm)
				}
				panic(e)
			}
		}()

		start := time.Now()
		c.Next()
		end := time.Now()
		latency := end.Sub(start).Nanoseconds() / 1000
		var handleMethod string
		stressTag := "-"
		if v := c.Value(internal.METHODKEY); v != nil {
			// 如果METHODKEY设置了,那它一定是string
			handleMethod = v.(string)
		}
		if v := c.Value(internal.STRESSKEY); v != nil {
			// 如果STRESSKEY被设置了，那他一定是string类型
			stressTag = v.(string)
		}

		statusCode := c.Writer.Status()
		tags := map[string]string{
			"status":        strconv.Itoa(statusCode),
			"handle_method": handleMethod,
			"from_cluster":  "default",
			"to_cluster":    env.Cluster(),
			"stress_tag":    stressTag,
		}
		if meshMode {
			tags["mesh"] = "1"
		}
		// https://wiki.bytedance.net/pages/viewpage.action?pageId=51348664
		success, newTags := successDecider(c, tags)
		if success {
			emitter.EmitTimer(succLatencyMetrics, latency, "", newTags)
			emitter.EmitCounter(succThroughputMetrics, 1, "", newTags)
		} else {
			emitter.EmitTimer(errLatencyMetrics, latency, "", newTags)
			emitter.EmitCounter(errThroughputMetrics, 1, "", newTags)
		}
		// emit framework metrics
		emitter.EmitCounter(frameWorkThroughputMetrics, 1, FRAMEWORK_METRICS_PREFIX, frameWorkMetricsTags)
	}
}

// Metrics returns MetricsWithSuccessDecider using default success decider
// (2xx and 3xx are success codes, error otherwise)
func Metrics(psm string, meshMode bool) gin.HandlerFunc {
	return MetricsWithSuccessDecider(psm, meshMode, defaultSuccessDecider)
}

// defaultSuccessDecider returns true if status code is either in 2xx or 3xx range.
func defaultSuccessDecider(c *gin.Context, tags map[string]string) (bool, map[string]string) {
	// determine the success by status code
	statusCode := c.Writer.Status()
	return statusCode >= 200 && statusCode < 400, tags
}

// EmitPanicCounter panic埋点, 业务可调用该方法统一埋点.
func EmitPanicCounter(psm string) {
	panicMetrics := fmt.Sprintf("%s.panic", psm)
	emitter.EmitCounter(panicMetrics, 1, "", make(map[string]string))
}

// EmitCurrentConnectionCount 记录当前建立的连接总数
func EmitCurrentConnectionCount(psm string, count int64) {
	countMetircs := fmt.Sprintf("%s.connection_count", psm)
	emitter.EmitStore(countMetircs, count, "", nil)
}
