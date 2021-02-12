package idgenerator

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"code.byted.org/gopkg/logs"
)

const defaultWaitTimeout = 100 * time.Millisecond

var TimeoutError = errors.New("Timeout error")

type IdGeneratorWrapper struct {
	Client *IdGeneratorClient
	count  int // 每次批量获取的条数

	idChan     chan int64
	empty      chan struct{}
	waitingCnt int32

	timeout time.Duration
	mu      sync.RWMutex
}

func (g *IdGeneratorWrapper) SetWaitTimeout(t time.Duration) {
	if t < 20*time.Millisecond {
		t = 20 * time.Millisecond
	}
	if t > 500*time.Millisecond {
		t = 500 * time.Millisecond
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	g.timeout = t
}

func (g *IdGeneratorWrapper) getWaitTimeout() time.Duration {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.timeout
}

func (g *IdGeneratorWrapper) Get() (int64, error) {
	atomic.AddInt32(&g.waitingCnt, 1)
	defer func() { atomic.AddInt32(&g.waitingCnt, -1) }()

	g.notify()

	waitTimeout := g.getWaitTimeout()
	select {
	case id := <-g.idChan:
		return id, nil
	case <-time.After(waitTimeout):
		return 0, TimeoutError
	}
}

func (g *IdGeneratorWrapper) notify() {
	select {
	case g.empty <- struct{}{}:
	default:
	}
}

func (g *IdGeneratorWrapper) start() {
	for {
		select {
		case <-g.empty:
			g.fetch()
		}
	}
}

func (g *IdGeneratorWrapper) fetch() {
	for {
		ids, err := g.Client.GenMulti(g.count)
		if err != nil {
			logs.Errorf("idgenerator genMulti failed : %v", err)
			time.Sleep(200 * time.Millisecond)
			continue
		}
		for _, id := range ids {
			g.idChan <- id
		}

		waitingCnt := atomic.LoadInt32(&g.waitingCnt)
		if waitingCnt == 0 {
			return
		}
	}
}

func newIdGeneratorWrapper(client *IdGeneratorClient, count int) *IdGeneratorWrapper {
	if count < 1 {
		count = 1
	}
	if count > 100 {
		count = 100
	}
	wrapper := IdGeneratorWrapper{
		Client:  client,
		count:   count,
		idChan:  make(chan int64, 100),
		empty:   make(chan struct{}, 1),
		timeout: defaultWaitTimeout,
	}
	go wrapper.start()
	return &wrapper
}
