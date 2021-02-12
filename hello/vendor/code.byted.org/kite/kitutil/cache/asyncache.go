package cache

import (
	"strings"
	"sync"
	"time"

	"code.byted.org/gopkg/logs"
)

// Options .
type Options struct {
	BlockIfFirst    bool
	RefreshDuration time.Duration
	Fetcher         func(key string) (interface{}, error)
	ErrHandler      func(key string, err error)
	ChangeHandler   func(key string, oldData, newData interface{})
	IsSame          func(key string, oldData, newData interface{}) bool
}

// Asyncache .
type Asyncache struct {
	sfg  *Group
	opt  Options
	data sync.Map
	exit chan struct{}
}

// NewAsyncache .
func NewAsyncache(opt Options) *Asyncache {
	c := &Asyncache{
		sfg:  &Group{},
		opt:  opt,
		exit: make(chan struct{}),
	}
	go c.refresher()
	return c
}

// Get .
func (c *Asyncache) Get(key string, defaultVal interface{}) interface{} {
	val, ok := c.data.Load(key)
	if ok {
		return val
	}

	if !c.opt.BlockIfFirst {
		c.data.Store(key, defaultVal)
		return defaultVal
	}

	// 避免启动时, 并发的对同一个key产生大量请求
	val, err := c.sfg.Do(key, func() (interface{}, error) {
		return c.opt.Fetcher(key)
	})
	if err != nil {
		if c.opt.ErrHandler != nil {
			c.opt.ErrHandler(key, err)
		} else {
			logs.Errorf("first fetch %s err: %s, default value: %v", key, err.Error(), val)
		}
		val = defaultVal
	}

	c.data.Store(key, val)
	return val
}

func (c *Asyncache) DelPrefix(prefix string) {
	c.data.Range(func(key, value interface{}) bool {
		s := key.(string)
		if strings.HasPrefix(s, prefix) {
			c.data.Delete(key)
		}
		return true
	})
}

// Dump .
func (c *Asyncache) Dump() map[string]interface{} {
	data := make(map[string]interface{})
	c.data.Range(func(key, value interface{}) bool {
		k, ok := key.(string)
		// 如果不是string就打印日志然后跳过
		if !ok {
			logs.Warn("invalid key: %v, type: %T is not string", k, k)
			return true
		}
		data[k] = value
		return true
	})
	return data
}

// Close .
func (c *Asyncache) Close() {
	close(c.exit)
}

func (c *Asyncache) refresher() {
	ch := time.Tick(c.opt.RefreshDuration)
	for {
		select {
		case <-c.exit:
			return
		case <-ch:
			c.refresh()
		}
	}
}

func (c *Asyncache) refresh() {
	oldData := c.Dump()
	for key, oldVal := range oldData {
		newVal, err := c.opt.Fetcher(key)
		if err != nil {
			if c.opt.ErrHandler != nil {
				go c.opt.ErrHandler(key, err)
			}
			continue
		}
		if c.opt.IsSame != nil && !c.opt.IsSame(key, oldVal, newVal) {
			if c.opt.ChangeHandler != nil {
				go c.opt.ChangeHandler(key, oldVal, newVal)
			}
		}

		c.data.Store(key, newVal)
	}
}
