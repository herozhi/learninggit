/*

Example:

```golang
package main

import (
	"fmt"
	"code.byted.org/kv/goredis"
	"code.byted.org/kv/redis-v6"
)

type middleware1 struct {}

func (m *middleware1) ProcessRequest(ctx *goredis.PipeExecContext) (error) {
	fmt.Printf("TEST_1_process_req: \n")
	return nil
}

func (m *middleware1) ProcessResponse(ctx *goredis.PipeExecContext, ret []redis.Cmder, err error) {
	fmt.Printf("TEST_1_process_resp: err=%v len=%v\n", err, len(ret))
	for _, cmd := range ret {
		fmt.Printf("TEST_1_process_resp: cmdName=[%v] args=[%v] err=[%v]\n", cmd.Name(), cmd.Args(), cmd.Err())
	}
	return
}

type middleware2 struct {}

func (m *middleware2) ProcessRequest(ctx *goredis.PipeExecContext) (error) {
	fmt.Printf("TEST_2_process_req: \n")
	return nil
}

func (m *middleware2) ProcessResponse(ctx *goredis.PipeExecContext, ret []redis.Cmder, err error) {
	fmt.Printf("TEST_2_process_resp: err=%v len=%v\n", err, len(ret))
	return
}

type middleware3 struct {}

func (m middleware3) ProcessRequest(ctx *goredis.PipeExecContext) (error) {
	fmt.Printf("TEST_3_process_req: \n")
	return nil
}

func (m middleware3) ProcessResponse(ctx *goredis.PipeExecContext, ret []redis.Cmder, err error) {
	fmt.Printf("TEST_3_process_resp: err=%v len=%v\n", err, len(ret))
	return
}

func main() {

	// register middleware
	goredis.UsePipeMiddleWare(&middleware1{}, &middleware2{})
	goredis.UsePipeMiddleWare(middleware3{})

	// bootstrap ....
}

```

*/

package goredis

import (
	redis "code.byted.org/kv/redis-v6"
)

// -- PipeExecContext
type PipeExecContext struct {
	pipeline *Pipeline
	data     map[string]interface{}
}

func (ctx *PipeExecContext) Pipeline() *Pipeline {
	return ctx.pipeline
}

// Ctx Set: non-concurrent safety
func (ctx *PipeExecContext) Set(key string, value interface{}) {
	if ctx.data == nil {
		ctx.data = make(map[string]interface{}, 1)
	}
	ctx.data[key] = value
}

// Ctx Get: non-concurrent safety
func (ctx *PipeExecContext) Get(key string) (interface{}, bool) {
	if ctx.data == nil {
		return nil, false
	}
	value, exists := ctx.data[key]
	return value, exists
}

// -- WrapProcessMiddleWare
type PipeExecMiddleWare interface {
	ProcessRequest(ctx *PipeExecContext) error
	ProcessResponse(ctx *PipeExecContext, ret []redis.Cmder, err error)
}

var pipeExecMiddleWares []PipeExecMiddleWare

func UsePipeMiddleWare(middleWares ...PipeExecMiddleWare) {
	pipeExecMiddleWares = append(pipeExecMiddleWares, middleWares...)
}
