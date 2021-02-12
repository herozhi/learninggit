package ginex

import (
	"context"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"

	"code.byted.org/gin/ginex/internal"
	"code.byted.org/gopkg/metainfo"
)

const (
	// RPCContextKey the key for cache RPC context
	RPCContextKey = "K_RPC_CTX"
)

// CtxKeyFilter the key will be ignored when generating context.Context
// using RPCContext if returned value is false
type CtxKeyFilter func(key string) bool

var ctxKeyFilter CtxKeyFilter

// 对于极少数没有使用Ctx的服务，在第一次调用getGinCtxMutex的时候才会用到这个全局的锁
// 不是一种优雅的实现，但是在不改gin原来代码情况之下暂时没想到更好的解决方案
// 根据benchmark，加上内存分配、循环等时间，结果如下：
// BenchmarkGetGinCtxMutexWithoutInitConcurrent-12	1000000	1372 ns/op
// BenchmarkGetGinCtxMutexWithoutInit-12	2000000	785 ns/op
var globalGinCtxMutex sync.Mutex

// RPCContext returns context information for rpc call
// It's a good choice to call this method at the beginning of handler function to
// avoid concurrent read and write gin.Context
func RPCContext(ginCtx *gin.Context) context.Context {
	ctx := context.Background()
	// 防止并发读写map
	m := getGinCtxMutex(ginCtx)
	m.RLock()
	if ginCtx.Keys != nil {
		for key, val := range ginCtx.Keys {
			if ctxKeyFilter != nil && !ctxKeyFilter(key) {
				continue
			}
			if key == internal.SPANCTXKEY {
				ctx = opentracing.ContextWithSpan(ctx, val.(opentracing.Span))
			} else if strings.HasPrefix(key, internal.RPC_PERSIST_PREFIX) {
				var newKey = key[len(internal.RPC_PERSIST_PREFIX):]
				newKey = strings.ToUpper(newKey)
				newKey = strings.Replace(newKey, "-", "_", -1)
				ctx = metainfo.WithPersistentValue(ctx, newKey, val.(string))
			} else {
				ctx = context.WithValue(ctx, key, val)
			}
		}
	}
	m.RUnlock()
	ctx = metainfo.SetMetaInfoFromHeader(ctx, metainfo.WrapHttpRequest(ginCtx.Request))
	return ctx
}

// RpcContext is depreciated for it has lint error, please use RPCContext.
// This API will be deleted in 2019.06
func RpcContext(ginCtx *gin.Context) context.Context {
	return RPCContext(ginCtx)
}

// CacheRPCContext returns context information for rpc call
// It's a good choice to call this method at the beginning of handler function to
// avoid concurrent read and write gin.Context
func CacheRPCContext(ginCtx *gin.Context) context.Context {
	m := getGinCtxMutex(ginCtx)
	m.RLock()
	ctx, exists := ginCtx.Get(RPCContextKey)
	m.RUnlock()
	if exists {
		return ctx.(context.Context)
	}

	ctx = RPCContext(ginCtx)

	// 这里有写，别处有读，所以需要加锁防止并发读写map
	// 但是这里没有避免创建多个Context的问题，并没有实现单例
	m.Lock()
	ginCtx.Set(RPCContextKey, ctx)
	m.Unlock()

	return ctx.(context.Context)
}

// CacheRpcContext depreciated
func CacheRpcContext(ginCtx *gin.Context) context.Context {
	return CacheRPCContext(ginCtx)
}

// GetGinCtxStressTag .
func GetGinCtxStressTag(ginCtx *gin.Context) string {
	return ginCtx.GetString(internal.STRESSKEY)
}

func getGinCtxMutex(ginCtx *gin.Context) *sync.RWMutex {
	mutex := getGinCtxMutexInterface(ginCtx)
	m, ok := mutex.(*sync.RWMutex)
	// 有一些用户不使用Ctx()进行初始化
	// 所以在这里需要通过全局锁锁住，双检查实现单例
	// 该实现不影响使用Ctx()进行初始化的用户
	if !ok {
		globalGinCtxMutex.Lock()
		mutex = getGinCtxMutexInterface(ginCtx)
		m, ok = mutex.(*sync.RWMutex)
		if !ok {
			fillRWMutexToGinRequestContext(ginCtx)
			mutex = getGinCtxMutexInterface(ginCtx)
			m, ok = mutex.(*sync.RWMutex)
			if !ok {
				// should not reach
				panic("should be sync.RWMutex here!")
			}
		}
		globalGinCtxMutex.Unlock()
	}
	return m
}

func fillRWMutexToGinRequestContext(ginCtx *gin.Context) {
	ginCtx.Request = ginCtx.Request.WithContext(context.WithValue(ginCtx.Request.Context(), internal.CONTEXT_MUTEX, &sync.RWMutex{}))
}

func getGinCtxMutexInterface(ginCtx *gin.Context) interface{} {
	return ginCtx.Request.Context().Value(internal.CONTEXT_MUTEX)
}

// amend handlerName of cache context
func amendCacheCtxHandlerName(ginCtx *gin.Context, handlerName string) {
	if val, exists := ginCtx.Get(RPCContextKey); exists {
		if ctx, ok := val.(context.Context); ok {
			if originHandlerName, ok := ctx.Value(internal.METHODKEY).(string); ok {
				if originHandlerName != handlerName {
					ginCtx.Set(RPCContextKey, context.WithValue(ctx, internal.METHODKEY, handlerName))
				}
			}
		}
	}
}

// SetCtxKeyFilter set key filter for RPCContext and related functions
func SetCtxKeyFilter(filter CtxKeyFilter) {
	ctxKeyFilter = filter
}
