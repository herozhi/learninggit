package whale

import (
	"code.byted.org/gopkg/metrics"
	"github.com/gin-gonic/gin"
)

func WhaleDeprecatedMiddleware() gin.HandlerFunc {
	metricsClient := metrics.NewDefaultMetricsClientV2("webarch.whale.antic.middleware", true)
	return func(ctx *gin.Context) {
		metricsClient.EmitCounter("request.ginex.err_req", 1)
		ctx.Next()
	}
}
