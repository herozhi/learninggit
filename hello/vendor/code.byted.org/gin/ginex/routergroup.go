package ginex

import (
	"code.byted.org/gin/ginex/internal"

	"github.com/gin-gonic/gin"
)

type RouterGroup struct {
	*gin.RouterGroup
}

func (group *RouterGroup) GroupEX(relativePath string, handlers ...gin.HandlerFunc) *RouterGroup {
	return &RouterGroup{RouterGroup: group.Group(relativePath, handlers...)}
}

// GETEX是GET的扩展版,增加了一个handlerName参数
// 当handler函数被decorator修饰或者是匿名函数时,直接获取HandleMethod得不到真正的handler名称
// 这种情况下使用-EX函数显示传入
func (group *RouterGroup) GETEX(relativePath string, handler gin.HandlerFunc, handlerName string) gin.IRoutes {
	internal.SetHandlerName(handler, handlerName)
	return group.GET(relativePath, handler)
}

func (group *RouterGroup) POSTEX(relativePath string, handler gin.HandlerFunc, handlerName string) gin.IRoutes {
	internal.SetHandlerName(handler, handlerName)
	return group.POST(relativePath, handler)
}

func (group *RouterGroup) PUTEX(relativePath string, handler gin.HandlerFunc, handlerName string) gin.IRoutes {
	internal.SetHandlerName(handler, handlerName)
	return group.PUT(relativePath, handler)
}

func (group *RouterGroup) DELETEEX(relativePath string, handler gin.HandlerFunc, handlerName string) gin.IRoutes {
	internal.SetHandlerName(handler, handlerName)
	return group.DELETE(relativePath, handler)
}

func (group *RouterGroup) HEADEX(relativePath string, handler gin.HandlerFunc, handlerName string) gin.IRoutes {
	internal.SetHandlerName(handler, handlerName)
	return group.HEAD(relativePath, handler)
}

func (group *RouterGroup) AnyEX(relativePath string, handler gin.HandlerFunc, handlerName string) gin.IRoutes {
	internal.SetHandlerName(handler, handlerName)
	return group.Any(relativePath, handler)
}

func (group *RouterGroup) HandleEX(httpMethod, relativePath string, handler gin.HandlerFunc, handlerName string) gin.IRoutes {
	internal.SetHandlerName(handler, handlerName)
	return group.Handle(httpMethod, relativePath, handler)
}
