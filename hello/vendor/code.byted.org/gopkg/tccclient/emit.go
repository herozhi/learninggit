package tccclient

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"code.byted.org/gopkg/metrics"
)

type emitStatus string

const (
	statusSucc     emitStatus = "success"
	statusErr      emitStatus = "failed"
	statusNotFound emitStatus = "notFound"

	defaultNotFoundKey = "_not_found_key_"
)

type emiterItem struct {
	succ     int64
	err      int64
	notFound int64
}

type emiter struct {
	serviceName string
	confspace   string
	cache       sync.Map // map[configKey]*emiterItem

	notFoundItem *emiterItem
}

func newEmiter(serviceName, confspace string) *emiter {
	e := emiter{
		serviceName:  serviceName,
		confspace:    confspace,
		notFoundItem: &emiterItem{},
	}
	e.cache.Store(defaultNotFoundKey, e.notFoundItem) // avoid too many not_found_key
	go e.emitCache()
	return &e
}

func (e *emiter) emit(err error, configKey, getType string) {
	status := statusSucc
	if err == ConfigNotFoundError {
		status = statusNotFound
	} else if err != nil {
		status = statusErr
	}

	if getType != GetTypeCache {
		e.doEmit(status, configKey, getType, 1)
		return
	}

	var item *emiterItem
	if itemI, ok := e.cache.Load(configKey); !ok {
		if status == statusNotFound {
			item = e.notFoundItem
		} else {
			itemI, _ = e.cache.LoadOrStore(configKey, &emiterItem{})
			item = itemI.(*emiterItem)
		}
	} else {
		item = itemI.(*emiterItem)
	}
	switch status {
	case statusSucc:
		atomic.AddInt64(&item.succ, 1)
	case statusErr:
		atomic.AddInt64(&item.err, 1)
	case statusNotFound:
		atomic.AddInt64(&item.notFound, 1)
	}
}

func (e *emiter) emitCache() {
	for {
		time.Sleep(10 * time.Second)
		e.cache.Range(func(keyI, valueI interface{}) bool {
			key := keyI.(string)
			curItem := valueI.(*emiterItem)
			succCnt := atomic.LoadInt64(&curItem.succ)
			errCnt := atomic.LoadInt64(&curItem.err)
			notFoundCnt := atomic.LoadInt64(&curItem.notFound)
			e.doEmit(statusSucc, key, GetTypeCache, succCnt)
			e.doEmit(statusErr, key, GetTypeCache, errCnt)
			e.doEmit(statusNotFound, key, GetTypeCache, notFoundCnt)
			atomic.AddInt64(&curItem.succ, -1*succCnt)
			atomic.AddInt64(&curItem.err, -1*errCnt)
			atomic.AddInt64(&curItem.notFound, -1*notFoundCnt)
			return true
		})
	}
}

func (e *emiter) doEmit(status emitStatus, configKey, getType string, count int64) {
	if count <= 0 {
		return
	}
	code := "0"
	switch status {
	case statusErr:
		code = "1"
	case statusNotFound:
		status = statusSucc
		code = "100"
	}

	key := fmt.Sprintf("client.%s.get_config.%s", e.serviceName, status)
	ts := []metrics.T{
		metrics.T{Name: "confspace", Value: e.confspace},
		metrics.T{Name: "config_key", Value: configKey},
		metrics.T{Name: "version", Value: clientVersion},
		metrics.T{Name: "api_version", Value: apiVersionV2},
		metrics.T{Name: "language", Value: "go"},
		metrics.T{Name: "code", Value: code},
		metrics.T{Name: "get_type", Value: getType},
	}

	metricsClient.EmitCounter(key, count, ts...)
}
