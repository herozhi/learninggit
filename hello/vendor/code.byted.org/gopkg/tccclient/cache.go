package tccclient

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

const dumpPath = "/var/tmp/tcc/data"

var now = func() time.Time {
	return time.Now()
}

// Item represents a record in the cache map
type Item struct {
	Value   string
	Expires time.Time
}

func (e *Item) Expired() bool {
	return now().After(e.Expires)
}

// Cache is a synchronised map of items
type Cache struct {
	mu    sync.RWMutex
	items map[string]*Item
}

// NewCache creates instance of Cache
func NewCache() *Cache {
	return &Cache{items: map[string]*Item{}}
}

// Set adds key => value to cache
func (c *Cache) Set(key string, item Item) {
	c.mu.Lock()
	c.items[key] = &item
	c.stepClean()
	c.mu.Unlock()
}

// Get returns value of key
func (c *Cache) Get(key string) *Item {
	c.mu.RLock()
	item := c.items[key]
	c.mu.RUnlock()
	if item == nil {
		return nil
	}
	return item
}

// Len returns items number of cache
func (c *Cache) Len() int {
	c.mu.RLock()
	n := len(c.items)
	c.mu.RUnlock()
	return n
}

func (c *Cache) stepClean() {
	const steps = 10
	n := 0
	for k, e := range c.items {
		if n >= steps {
			break
		}
		if now().After(e.Expires.Add(10 * time.Minute)) {
			// key has expired for a long time
			delete(c.items, k)
		}
		n++
	}
}

type ConfigCache struct {
	serviceName string
	confspace   string
	cacheTime   time.Duration
	dumpPath    string
	mu          sync.RWMutex
	sf          singleflight.Group
	data        *Data
	lastErr     error
	expiredAt   time.Time
	accessAt    int64
}

func NewConfigCache(serviceName, confspace string, cacheTime time.Duration) *ConfigCache {
	return &ConfigCache{
		serviceName: serviceName,
		confspace:   confspace,
		cacheTime:   cacheTime,
	}
}

func (c *ConfigCache) Set(data *Data, err error) {
	c.mu.Lock()
	c.data = data
	c.lastErr = err
	c.expiredAt = now().Add(c.cacheTime)
	c.mu.Unlock()
	go c.Dump(data, err)
}

func (c *ConfigCache) Get() (*Data, error, bool) {
	c.mu.RLock()
	data := c.data
	lastErr := c.lastErr
	expiredAt := c.expiredAt
	c.mu.RUnlock()
	atomic.StoreInt64(&c.accessAt, now().Unix())
	return data, lastErr, now().After(expiredAt)
}

func (c *ConfigCache) GetWithAccessAt() (*Data, error, int64) {
	c.mu.RLock()
	data := c.data
	lastErr := c.lastErr
	c.mu.RUnlock()
	accessAt := atomic.LoadInt64(&c.accessAt)
	return data, lastErr, accessAt
}

func (c *ConfigCache) Dump(data *Data, err error) {
	c.sf.Do(c.serviceName+"@"+c.confspace, func() (interface{}, error) {
		c.dump(data, err)
		return nil, nil
	})
}

func (c *ConfigCache) dump(data *Data, err error) {
	if c.serviceName == "" || c.confspace == "" {
		return
	}

	type DumpData struct {
		Data       *Data  `json:"data"`
		LastErr    string `json:"last_err"`
		ModifyTime int64  `json:"modify_time"`
	}

	dumpData := DumpData{
		Data:       data,
		ModifyTime: time.Now().Unix(),
	}
	if err != nil {
		dumpData.LastErr = err.Error()
	}
	dumpDataBytes, err := json.Marshal(dumpData)
	if err != nil {
		return
	}

	dumpDir := dumpPath + "/" + c.serviceName
	if os.MkdirAll(dumpDir, os.ModePerm) != nil {
		return
	}

	dumpFile := dumpDir + "/" + c.confspace
	dumpFileTmp := fmt.Sprintf("%s_%d", dumpFile, os.Getpid())
	f, err := os.OpenFile(dumpFileTmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return
	}
	defer f.Close()
	dumpDataBytes = append(dumpDataBytes, '\n')
	f.Write(dumpDataBytes)
	os.Rename(dumpFileTmp, dumpFile)
}
