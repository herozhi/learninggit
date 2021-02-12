package tccclient

import (
	"context"
	"sync"
)

type ParserCache struct {
	tccValueCache  map[string]string
	tccKeyVersion  map[string]string
	tccResultCache map[string]interface{}
	mu             sync.RWMutex
}

func NewParserCache() *ParserCache {
	return &ParserCache{
		tccValueCache:  make(map[string]string),
		tccKeyVersion:  make(map[string]string),
		tccResultCache: make(map[string]interface{}),
		mu:             sync.RWMutex{},
	}
}

func (ps *ParserCache) Set(key, value, versionCode string, result interface{}) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.tccValueCache[key] = value
	ps.tccKeyVersion[key] = versionCode
	ps.tccResultCache[key] = result
}

func (ps *ParserCache) Get(key string) (string, string, interface{}, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	value, exist := ps.tccValueCache[key]
	return value, ps.tccKeyVersion[key], ps.tccResultCache[key], exist
}

type TCCParser func(value string, err error, cacheResult interface{}) (interface{}, error)

func (client *Client) GetWithParser(key string, parser TCCParser) (interface{}, error) {
	value, expired, err := client.getWithCache(key)
	cacheValue, _, cacheResult, exist := client.parserCache.Get(key)
	if !expired && exist {
		return cacheResult, nil
	}
	if err == nil && exist && cacheValue == value {
		// expired but value not update
		return cacheResult, nil
	}
	result, err := parser(value, err, cacheResult)
	if err == nil {
		client.parserCache.Set(key, value, "", result)
	}
	return result, err
}

func (client *ClientV2) GetWithParser(ctx context.Context, key string, parser TCCParser) (interface{}, error) {
	value, versionCode, err := client.getWithCache(ctx, key)
	_, cacheVersion, cacheResult, exist := client.parserCache.Get(key)
	if exist && versionCode == cacheVersion {
		return cacheResult, nil
	}
	result, err := parser(value, err, cacheResult)
	if err == nil {
		client.parserCache.Set(key, value, versionCode, result)
	}
	return result, err
}
