package goredis

import (
	"errors"
	"sync"
	"time"

	redis "code.byted.org/kv/redis-v6"
)

type Pipeline struct {
	redis.Pipeliner
	c                  *Client
	cluster            string
	psm                string
	metricsServiceName string
	name               string
}

var ppool = &sync.Pool{New: func() interface{} { return make(map[string]int, 5) }}

// func (p *Pipeline) SetPSM

// Exec executes all previously queued commands using one client-server roundtrip.
//
// Exec returns list of commands and error.
// miss is not a error in pipeline.
// you should use Cmder.Err() == redis.Nil to find whether miss occur or not
//
// After Commit, you should use Close() to close the pipeline releasing open resources.
func (p *Pipeline) Exec() ([]redis.Cmder, error) {
	start := time.Now().UnixNano()
	var resErr error
	ctx := &PipeExecContext{
		pipeline: p,
	}

	// 0. process middleware request
	processIndex := 0
	for ; resErr == nil && processIndex < len(pipeExecMiddleWares); processIndex++ {
		resErr = pipeExecMiddleWares[processIndex].ProcessRequest(ctx)
	}

	//  1. exec
	var cmder []redis.Cmder

	if resErr == nil {
		cmder = p.exec()
	} else {
		// save process request error to cmds result
		cmder = p.Pipeliner.Cmds()
		for _, c := range cmder {
			c.SetErr(resErr)
		}
	}

	pipelineCmdNum := len(cmder)
	if pipelineCmdNum == 0 && resErr == nil {
		// should not run here
		resErr = errors.New("pipeline cmd num is 0")
	}

	// 2. metrics calculate
	cmdErrorCounter := ppool.Get().(map[string]int)
	cmdSuccessCounter := ppool.Get().(map[string]int)
	for k, _ := range cmdErrorCounter {
		delete(cmdErrorCounter, k)
	}
	for k, _ := range cmdSuccessCounter {
		delete(cmdSuccessCounter, k)
	}

	for _, res := range cmder {
		cmdStr := res.Name()
		// one or more miss occur in pipeline, and we think miss is not a error in pipeline
		if res.Err() != nil && res.Err() != redis.Nil {
			counter, ok := cmdErrorCounter[cmdStr]
			if ok {
				cmdErrorCounter[cmdStr] = counter + 1
			} else {
				cmdErrorCounter[cmdStr] = 1
			}
			if resErr == nil {
				resErr = res.Err()
			}
		} else {
			counter, ok := cmdSuccessCounter[cmdStr]
			if ok {
				cmdSuccessCounter[cmdStr] = counter + 1
			} else {
				cmdSuccessCounter[cmdStr] = 1
			}
		}
	}

	// 3. process response
	for index := processIndex - 1; index >= 0; index-- {
		pipeExecMiddleWares[index].ProcessResponse(ctx, cmder, resErr)
	}

	// 3. metrics emit
	latency := (time.Now().UnixNano() - start) / 1000

	// Aggregate pipeline cmd metrics by cmdStr
	// separate cmd
	for cmdStr, counter := range cmdSuccessCounter {
		addCallMetrics(p.c.ctx, cmdStr, -1, nil, p.c.cluster, p.c.psm, p.c.metricsServiceName, counter)
	}
	for cmdStr, counter := range cmdErrorCounter {
		addCallMetrics(p.c.ctx, cmdStr, -1, resErr, p.c.cluster, p.c.psm, p.c.metricsServiceName, counter)
	}

	addCallMetrics(p.c.ctx, "pipeline", latency, resErr, p.c.cluster, p.c.psm, p.c.metricsServiceName, 1)
	if pipelineCmdNum > 500 {
		addCallMetrics(p.c.ctx, "big_pipeline", -1, nil, p.c.cluster, p.c.psm, p.c.metricsServiceName, 1)
	}

	ppool.Put(cmdErrorCounter)
	ppool.Put(cmdSuccessCounter)

	// --

	return cmder, resErr
}

func (p *Pipeline) exec() (ret []redis.Cmder) {
	// degredate
	/*cmds := p.Pipeliner.Cmds()
	notDegCmds := make([]redis.Cmder, 0, len(cmds))
	for _, c := range cmds {
		if cmdDegredated(p.metricsServiceName, c.Name()) {
			c.SetErr(ErrDegradated)
		} else {
			notDegCmds = append(notDegCmds, c)
		}
	}
	p.Pipeliner.SetCmds(notDegCmds)
	*/

	// hack for stress tag
	var stressCmds []redis.Cmder
	if prefix, ok := isStressTest(p.c.ctx); ok {
		stressCmds = make([]redis.Cmder, 0, len(p.Pipeliner.Cmds()))
		isAbase := isAbaseCluster(p.c.Cluster())
		for _, c := range p.Pipeliner.Cmds() {
			stressCmds = append(stressCmds, convertStressCMD(isAbase, prefix, c))
		}
		// modify pipeline's cmds
		p.Pipeliner.SetCmds(stressCmds)
	}

	cmder, _ := p.Pipeliner.Exec()

	return cmder
}

func (p *Pipeline) Cluster() string {
	return p.cluster
}

func (p *Pipeline) PSM() string {
	return p.psm
}

func (p *Pipeline) MetricsServiceName() string {
	return p.metricsServiceName
}

func (p *Pipeline) Name() string {
	return p.name
}

func (p *Pipeline) Client() *Client {
	return p.c
}
