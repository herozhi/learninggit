package ginex

import (
	"github.com/gin-gonic/gin"

	"code.byted.org/gin/ginex/accesslog"
	"code.byted.org/gin/ginex/apimetrics"
	"code.byted.org/gin/ginex/compress"
	"code.byted.org/gin/ginex/ctx"
	"code.byted.org/gin/ginex/stress"
	"code.byted.org/gin/ginex/throttle"
	"code.byted.org/gin/ginex/whale"
	"code.byted.org/gopkg/logs"
)

// MiddlewareConfig contains default ginex middlewares.
//
// It can be used to substitute or remove some of default middlewares provided by ginex.
// `nil` middlewares are ignored and will not be used.
type MiddlewareConfig struct {
	RecoveryMiddleware gin.HandlerFunc
	ContextMiddleware  gin.HandlerFunc

	DyeForceTraceMiddleware gin.HandlerFunc
	OpentracingMiddleware   gin.HandlerFunc

	AccessLogMiddleware      gin.HandlerFunc
	MetricsMiddleware        gin.HandlerFunc
	StressSwitcherMiddleware gin.HandlerFunc
	ThrottleMiddleware       gin.HandlerFunc

	WhaleAnticrawlMiddleware gin.HandlerFunc

	CompressMiddleware gin.HandlerFunc
}

func (mc MiddlewareConfig) MiddlewareList() []gin.HandlerFunc {
	allMiddlewares := []gin.HandlerFunc{mc.RecoveryMiddleware, mc.ContextMiddleware, mc.DyeForceTraceMiddleware,
		mc.OpentracingMiddleware, mc.AccessLogMiddleware, mc.MetricsMiddleware, mc.StressSwitcherMiddleware,
		mc.ThrottleMiddleware, mc.WhaleAnticrawlMiddleware, mc.CompressMiddleware}
	nonNilMiddlewares := make([]gin.HandlerFunc, 0, len(allMiddlewares))
	for _, mw := range allMiddlewares {
		nonNilMiddlewares = mc.appendIfNonNil(nonNilMiddlewares, mw)
	}
	return nonNilMiddlewares
}

// appendIfNonNil returns a list with mw appended, if it's non-nil. If mw is nil, unchanged mwList is returned.
func (mc MiddlewareConfig) appendIfNonNil(mwList []gin.HandlerFunc, mw gin.HandlerFunc) []gin.HandlerFunc {
	if mw == nil {
		return mwList
	}
	return append(mwList, mw)
}

// DefaultMiddlewareConfig returns default middleware configuration, the same one that
// can is used by using Default() function.
//
// To work correctly, it *MUST* be called after calling Init()!
func DefaultMiddlewareConfig() MiddlewareConfig {
	config := MiddlewareConfig{
		RecoveryMiddleware: gin.RecoveryWithWriter(&recoverWriter{}),
		ContextMiddleware:  ctx.Ctx(),

		AccessLogMiddleware: accesslog.AccessLog(accessLogger),
		MetricsMiddleware:   apimetrics.Metrics(PSM(), ServiceMeshMode()),
		CompressMiddleware:  compress.Compress(PSM()),
	}
	if !appConfig.ServiceMeshMode {
		config.StressSwitcherMiddleware = stress.StressSwitcher(PSM(), LocalCluster())
		config.ThrottleMiddleware = throttle.Throttle(PSM())
		logs.Infof("Ginex init middlewares['throttle', 'stress'] on non-mesh mode")
	}
	if appConfig.EnableTracing {
		config.DyeForceTraceMiddleware = DyeForceTraceHandler()
		config.OpentracingMiddleware = OpentracingHandler()
	}
	if EnableWhaleAnticrawl() {
		logs.Warnf("[Whale] The newest version Ginex removes supports for Whale. Please refer to https://wiki.bytedance.net/pages/viewpage.action?pageId=115955939 for better solutions.")
		config.WhaleAnticrawlMiddleware = whale.WhaleDeprecatedMiddleware()
	}
	return config
}
