package asynccache

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	sf "code.byted.org/gopkg/singleflight"
)

// Options controls the behavior of AsyncCache.
type Options struct {
	RefreshDuration time.Duration
	Fetcher         func(key string) (interface{}, error)

	// If EnableExpire is true, ExpireDuration MUST be set.
	EnableExpire   bool
	ExpireDuration time.Duration

	ErrorHandler  func(key string, err error)
	ChangeHandler func(key string, oldData, newData interface{})
	DeleteHandler func(key string, oldData interface{})

	IsSame     func(key string, oldData, newData interface{}) bool
	ErrLogFunc func(str string)
}

// AsyncCache .
type AsyncCache interface {
	// SetDefault sets the default value of given key if it is new to the cache.
	// It is useful for cache warming up.
	// Param val should not be nil.
	SetDefault(key string, val interface{}) (exist bool)

	// Get tries to fetch a value corresponding to the given key from the cache.
	// If error occurs during the first time fetching, it will be cached until the
	// sequential fetching triggered by the refresh goroutine succeed.
	Get(key string) (val interface{}, err error)

	// GetOrSet tries to fetch a value corresponding to the given key from the cache.
	// If the key is not yet cached or error occurs, the default value will be set.
	GetOrSet(key string, defaultVal interface{}) (val interface{})

	// Dump dumps all cache entries.
	// This will not cause expire to refresh.
	Dump() map[string]interface{}

	// DeleteIf deletes cached entries that match the `shouldDelete` predicate.
	DeleteIf(shouldDelete func(key string) bool)

	// Close closes the async cache.
	// This should be called when the cache is no longer needed, or may lead to resource leak.
	Close()
}

// asyncCache .
type asyncCache struct {
	sfg           sf.Group
	opt           Options
	data          sync.Map
	refreshTicker *time.Ticker
	expireTicker  *time.Ticker
}

type entry struct {
	val    atomic.Value
	expire int32 // 0 means useful, 1 will expire
	err    Error
}

func (e *entry) Store(x interface{}, err error) {
	if x != nil {
		e.val.Store(x)
	} else {
		e.val = atomic.Value{}
	}
	e.err.Store(err)
}

func (e *entry) Touch() {
	atomic.StoreInt32(&e.expire, 0)
}

// NewAsyncCache creates an AsyncCache.
func NewAsyncCache(opt Options) AsyncCache {
	c := &asyncCache{
		sfg: sf.Group{},
		opt: opt,
	}
	if c.opt.ErrLogFunc == nil {
		c.opt.ErrLogFunc = func(str string) {
			log.Println(str)
		}
	}
	if c.opt.EnableExpire {
		if c.opt.ExpireDuration == 0 {
			panic("asynccache: invalid ExpireDuration")
		}
		go c.expirer()
	}
	go c.refresher()
	return c
}

// SetDefault sets the default value of given key if it is new to the cache.
func (c *asyncCache) SetDefault(key string, val interface{}) bool {
	ety := &entry{}
	ety.Store(val, nil)
	actual, exist := c.data.LoadOrStore(key, ety)
	if exist {
		actual.(*entry).Touch()
	}
	return exist
}

// Get tries to fetch a value corresponding to the given key from the cache.
// If error occurs during in the first time fetching, it will be cached until the
// sequential fetchings triggered by the refresh goroutine succeed.
func (c *asyncCache) Get(key string) (val interface{}, err error) {
	var ok bool
	val, ok = c.data.Load(key)
	if ok {
		e := val.(*entry)
		e.Touch()
		return e.val.Load(), e.err.Load()
	}

	val, err, _ = c.sfg.Do(key, func() (v interface{}, e error) {
		v, e = c.opt.Fetcher(key)
		ety := &entry{}
		ety.Store(v, e)
		c.data.Store(key, ety)
		return
	})
	return
}

// GetOrSet tries to fetch a value corresponding to the given key from the cache.
// If the key is not yet cached or fetching failed, the default value will be set.
func (c *asyncCache) GetOrSet(key string, def interface{}) (val interface{}) {
	if v, ok := c.data.Load(key); ok {
		e := v.(*entry)
		if e.err.Load() != nil {
			ety := &entry{}
			ety.Store(def, nil)
			c.data.Store(key, ety)
			return def
		}
		e.Touch()
		return e.val.Load()
	}

	val, _, _ = c.sfg.Do(key, func() (interface{}, error) {
		v, e := c.opt.Fetcher(key)
		if e != nil {
			v = def
		}
		ety := &entry{}
		ety.Store(v, nil)
		c.data.Store(key, ety)
		return v, nil
	})
	return
}

// Dump dumps all cached entries.
func (c *asyncCache) Dump() map[string]interface{} {
	data := make(map[string]interface{})
	c.data.Range(func(key, val interface{}) bool {
		k, ok := key.(string)
		if !ok {
			c.opt.ErrLogFunc(fmt.Sprintf("invalid key: %v, type: %T is not string", k, k))
			c.data.Delete(key)
			return true
		}
		data[k] = val.(*entry).val.Load()
		return true
	})
	return data
}

// DeleteIf deletes cached entries that match the `shouldDelete` predicate.
func (c *asyncCache) DeleteIf(shouldDelete func(key string) bool) {
	c.data.Range(func(key, value interface{}) bool {
		s := key.(string)
		if shouldDelete(s) {
			if c.opt.DeleteHandler != nil {
				go c.opt.DeleteHandler(s, value)
			}
			c.data.Delete(key)
		}
		return true
	})
}

// Close stops the background refresh goroutine.
func (c *asyncCache) Close() {
	c.refreshTicker.Stop()
	if c.opt.EnableExpire {
		c.expireTicker.Stop()
	}
}

func (c *asyncCache) refresher() {
	c.refreshTicker = time.NewTicker(c.opt.RefreshDuration)
	for range c.refreshTicker.C {
		c.refresh()
	}
}

func (c *asyncCache) expirer() {
	c.expireTicker = time.NewTicker(c.opt.ExpireDuration)
	for range c.expireTicker.C {
		c.expire()
	}
}

func (c *asyncCache) expire() {
	c.data.Range(func(key, value interface{}) bool {
		k, ok := key.(string)
		if !ok {
			c.opt.ErrLogFunc(fmt.Sprintf("invalid key: %v, type: %T is not string", k, k))
			c.data.Delete(key)
			return true
		}
		e, ok := value.(*entry)
		if !ok {
			c.opt.ErrLogFunc(fmt.Sprintf("invalid key: %v, type: %T is not entry", k, value))
			c.data.Delete(key)
			return true
		}
		if !atomic.CompareAndSwapInt32(&e.expire, 0, 1) {
			if c.opt.DeleteHandler != nil {
				go c.opt.DeleteHandler(k, value)
			}
			c.data.Delete(key)
		}

		return true
	})
}

func (c *asyncCache) refresh() {
	c.data.Range(func(key, value interface{}) bool {
		k, ok := key.(string)
		if !ok {
			c.opt.ErrLogFunc(fmt.Sprintf("invalid key: %v, type: %T is not string", k, k))
			c.data.Delete(key)
			return true
		}
		e, ok := value.(*entry)
		if !ok {
			c.opt.ErrLogFunc(fmt.Sprintf("invalid key: %v, type: %T is not entry", k, value))
			c.data.Delete(key)
			return true
		}

		newVal, err := c.opt.Fetcher(k)
		if err != nil {
			if c.opt.ErrorHandler != nil {
				go c.opt.ErrorHandler(k, err)
			}
			if e.err.Load() != nil {
				e.err.Store(err)
			}
			return true
		}

		if c.opt.IsSame != nil && !c.opt.IsSame(k, e.val.Load(), newVal) {
			if c.opt.ChangeHandler != nil {
				go c.opt.ChangeHandler(k, e.val.Load(), newVal)
			}
		}

		e.Store(newVal, err)
		return true
	})
}
