package throttle

import (
	"fmt"
	"path"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"code.byted.org/gin/ginex/configstorer"
	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/logs"
	"code.byted.org/kite/kite/ratelimit"
)

func InitThrottle() {
	// refresh remote config
	buildMSKey := func(prefix string) string {
		psm := env.PSM()
		cluster := env.Cluster()

		p := path.Join(prefix, psm)
		if cluster != "" && cluster != "default" {
			p = path.Join(p, cluster)
		}
		return p
	}

	getETCDInt := func(key string, defaultNum int) int {
		val, err := configstorer.GetOrDefault(key, fmt.Sprintf("%v", defaultNum))
		if err != nil {
			logs.Errorf("GINEX: configstorer get key=%s err:%+v", key, err)
			return defaultNum
		}
		num, err := strconv.Atoi(val)
		if err != nil {
			logs.Errorf("GINEX: invalid etcd val=%s, key=%s", val, key)
			return defaultNum
		}
		if num <= 0 {
			logs.Errorf("GINEX: invalid etcd num val=%s, key=%s", val, key)
			return defaultNum
		}
		return num
	}

	go func() {
		lastQPSLim := DEFAULT_QPS_LIMIT
		lastConLim := DEFAULT_MAX_CON

		qpsKey := buildMSKey("/kite/limit/qps")
		conKey := buildMSKey("/kite/limit/conn")
		for range time.Tick(time.Second * 10) {
			newQPS := getETCDInt(qpsKey, lastQPSLim)
			newCon := getETCDInt(conKey, lastConLim)

			if newQPS != lastQPSLim {
				globalLimiter.UpdateQPSLimit(int64(newQPS))
				logs.Infof("GINEX: qps limit change from %v to %v", lastQPSLim, newQPS)
				lastQPSLim = newQPS
			}
			if newCon != lastConLim {
				globalLimiter.UpdateConnLimit(int64(newCon))
				logs.Infof("GINEX: conn limit change from %v to %v", lastConLim, newCon)
				lastConLim = newCon
			}
		}
	}()
}

var (
	globalLimiter = newLimiter(DEFAULT_QPS_LIMIT, DEFAULT_MAX_CON)
)

type limiter struct {
	conLimiter *ratelimit.ConcurrencyLimter
	qpsLimit   int64
	qpsLimiter *ratelimit.Bucket
	qpsLock    sync.RWMutex // for update qps limit
}

func newLimiter(qpsLim, maxConLim int64) *limiter {
	interval := time.Second / time.Duration(qpsLim)
	return &limiter{
		qpsLimit:   qpsLim,
		conLimiter: ratelimit.NewConcurrencyLimter(maxConLim),
		qpsLimiter: ratelimit.NewBucket(interval, qpsLim),
	}
}

func (ol *limiter) TakeCon() bool {
	return ol.conLimiter.TakeOne()
}

func (ol *limiter) ReleaseCon() {
	ol.conLimiter.ReleaseOne()
}

func (ol *limiter) UpdateConnLimit(lim int64) {
	ol.conLimiter.UpdateLimit(lim)
}

func (ol *limiter) ConnNow() int64 {
	return ol.conLimiter.Now()
}

func (ol *limiter) ConnLimit() int64 {
	return ol.conLimiter.Limit()
}

func (ol *limiter) TakeQPS() bool {
	ol.qpsLock.RLock()
	ok := ol.qpsLimiter.TakeAvailable(1) == 1
	ol.qpsLock.RUnlock()
	return ok
}

func (ol *limiter) UpdateQPSLimit(lim int64) {
	atomic.StoreInt64(&ol.qpsLimit, lim)
	interval := time.Second / time.Duration(lim)
	ol.qpsLock.Lock()
	ol.qpsLimiter = ratelimit.NewBucket(interval, lim)
	ol.qpsLock.Unlock()
}

func (ol *limiter) QPSLimit() int64 {
	return atomic.LoadInt64(&ol.qpsLimit)
}
