/*
	Wrappers with chains
*/

package goredis

import (
	"time"

	redis "code.byted.org/kv/redis-v6"
)

type ProcessFunc func(cmd redis.Cmder) error

// ----- user custom WrapProcess Middleware
/*
	Example:
		type middleware1 struct{}

		func (m *middleware1) ProcessRequest(ctx *goredis.WrapProcessContext, cmder redis.Cmder) (redis.Cmder, error){
			fmt.Printf("TEST_1_PROCESS_REQUEST: redis[%v] cmdName[%v]\n", ctx.Client().MetricsServiceName(), cmder.Name())
			return cmder, nil
		}

		func (m *middleware1) ProcessResponse(ctx *goredis.WrapProcessContext, cmder redis.Cmder, err error) {
			key := "hello"
			value, _ := ctx.Get(key)
			fmt.Printf("TEST_1_PROCESS_RESPONSE: key[%v] value[%v]\n", key, value)
			fmt.Printf("TEST_1_PROCESS_RESPONSE: redis[%v] cmdName[%v] err[%v]\n", ctx.Client().MetricsServiceName(), cmder.Name(), err)
		}

		type middleware2 struct{}

		func (m *middleware2) ProcessRequest(ctx *goredis.WrapProcessContext, cmder redis.Cmder) (redis.Cmder, error){
			ctx.Set("hello", "kitty")
			fmt.Printf("TEST_2_PROCESS_REQUEST: redis[%v] cmdName[%v]\n", ctx.Client().MetricsServiceName(), cmder.Name())
			return cmder, nil
		}

		func (m *middleware2) ProcessResponse(ctx *goredis.WrapProcessContext, cmder redis.Cmder, err error){
			key := "hello"
			value, _ := ctx.Get(key)
			fmt.Printf("TEST_2_PROCESS_RESPONSE: key[%v] value[%v]\n", key, value)
			fmt.Printf("TEST_2_PROCESS_RESPONSE: redis[%v] cmdName[%v] err[%v]\n", ctx.Client().MetricsServiceName(), cmder.Name(), err)

		}

		type middleware3 struct{}

		func (m middleware3) ProcessRequest(ctx *goredis.WrapProcessContext, cmder redis.Cmder) (redis.Cmder, error){
			fmt.Printf("TEST_3_PROCESS_REQUEST: redis[%v] cmdName[%v]\n", ctx.Client().MetricsServiceName(), cmder.Name())
			return cmder, nil
		}

		func (m middleware3) ProcessResponse(ctx *goredis.WrapProcessContext, cmder redis.Cmder, err error){
			key := "hello"
			value, _ := ctx.Get(key)
			fmt.Printf("TEST_3_PROCESS_RESPONSE: key[%v] value[%v]\n", key, value)
			fmt.Printf("TEST_3_PROCESS_RESPONSE: redis[%v] cmdName[%v] err[%v]\n", ctx.Client().MetricsServiceName(), cmder.Name(), err)
		}

		goredis.UseMiddleWare(&middleware1{}, &middleware2{})
		goredis.UseMiddleWare(middleware3{})

}
*/

// -- WrapProcessContext
type WrapProcessContext struct {
	client *Client
	data   map[string]interface{}
}

func (ctx *WrapProcessContext) Client() *Client {
	return ctx.client
}

// Ctx Set: non-concurrent safety
func (ctx *WrapProcessContext) Set(key string, value interface{}) {
	if ctx.data == nil {
		ctx.data = make(map[string]interface{}, 1)
	}
	ctx.data[key] = value
}

// Ctx Get: non-concurrent safety
func (ctx *WrapProcessContext) Get(key string) (interface{}, bool) {
	if ctx.data == nil {
		return nil, false
	}
	value, exists := ctx.data[key]
	return value, exists
}

// -- WrapProcessMiddleWare
type WrapProcessMiddleWare interface {
	ProcessRequest(ctx *WrapProcessContext, cmder redis.Cmder) (redis.Cmder, error)
	ProcessResponse(ctx *WrapProcessContext, cmder redis.Cmder, err error)
}

// -- internal WrapProcessMiddleWare Start --
type metricsWrapProcessMiddleWare struct{}

var metricsCtxStartTimeKey = "metrics.st"

func (m *metricsWrapProcessMiddleWare) ProcessRequest(ctx *WrapProcessContext, cmd redis.Cmder) (redis.Cmder, error) {
	// degradate
	/*
		if cmdDegredated(c.metricsServiceName, cmd.Name()) {
			cmd.SetErr(ErrDegradated)
			return ErrDegradated
		}
	*/
	// if stress rpc, hack args
	if prefix, ok := isStressTest(ctx.client.ctx); ok {
		isAbase := isAbaseCluster(ctx.client.Cluster())
		cmd = convertStressCMD(isAbase, prefix, cmd)
	}
	startTime := time.Now().UnixNano()
	ctx.Set(metricsCtxStartTimeKey, startTime)

	return cmd, nil
}

func (m *metricsWrapProcessMiddleWare) ProcessResponse(ctx *WrapProcessContext, cmd redis.Cmder, err error) {
	iStartTime, exists := ctx.Get(metricsCtxStartTimeKey)
	if !exists {
		return
	}
	startTime, ok := iStartTime.(int64)
	if !ok {
		return
	}

	latency := (time.Now().UnixNano() - startTime) / 1000
	addCallMetrics(ctx.client.ctx, cmd.Name(), latency, cmd.Err(), ctx.client.cluster, ctx.client.psm, ctx.client.metricsServiceName, 1)
}

// -- internal WrapProcessMiddleWare End --

var wrapProcessMiddleWares []WrapProcessMiddleWare

func UseMiddleWare(middleWares ...WrapProcessMiddleWare) {
	wrapProcessMiddleWares = append(wrapProcessMiddleWares, middleWares...)
}

func middleWareWrapProcess(client *Client, next ProcessFunc) ProcessFunc {
	middleWares := make([]WrapProcessMiddleWare, 0)

	// 1. force metrics middleWare at first
	middleWares = append(middleWares, &metricsWrapProcessMiddleWare{})

	// 2. opentracing middleware
	middleWares = append(middleWares, &opentracingMiddleWare{})

	// 3. add user defined middleWares
	if len(wrapProcessMiddleWares) > 0 {
		middleWares = append(middleWares, wrapProcessMiddleWares...)
	}

	return func(cmd redis.Cmder) (err error) {
		ctx := &WrapProcessContext{client: client}
		processIndex := 0

		// process request
		for ; err == nil && processIndex < len(middleWares); processIndex++ {
			cmd, err = middleWares[processIndex].ProcessRequest(ctx, cmd)
		}

		cmd.SetErr(err)
		if err == nil {
			err = next(cmd)
		}

		// process response
		for index := processIndex - 1; index >= 0; index-- {
			middleWares[index].ProcessResponse(ctx, cmd, err)
		}

		return err
	}
}

func chainWrapProcessMiddleWares(client *Client) func(func(cmd redis.Cmder) error) func(cmd redis.Cmder) error {
	return func(next func(cmd redis.Cmder) error) func(cmd redis.Cmder) error {
		next = middleWareWrapProcess(client, next)
		return next
	}
}
