package configstorer

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"code.byted.org/gopkg/asynccache"
	etcdutil "code.byted.org/gopkg/etcd_util"
)

var (
	configCache asynccache.AsyncCache
	mutex       sync.Mutex
	// ErrEmpty indicates that the interface return empty val
	ErrEmpty = errors.New("get empty val")
)

type Fetcher func(key string) (interface{}, error)

const (
	keyNotFound = "__cs_KEY_NOT_FOUND__"
)

func configFetcher(key string) (interface{}, error) {
	cli, err := etcdutil.GetDefaultClient()
	if err != nil {
		return "", err
	}

	node, err := cli.Get(context.Background(), key, nil)

	if err != nil {
		if isEtcdKeyNonexist(err) {
			return keyNotFound, nil
		}
		return "", err
	}
	return node.Node.Value, nil
}

func InitStorer() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Create config storer of etcd error. %v", r)
		}
	}()

	if configCache == nil {
		mutex.Lock()
		defer mutex.Unlock()
		if configCache == nil {
			opt := asynccache.Options{
				Fetcher:         configFetcher,
				RefreshDuration: time.Duration((3000 + rand.Intn(2000))) * time.Millisecond,
			}
			configCache = asynccache.NewAsyncCache(opt)
			etcdutil.SetRequestTimeout(50 * time.Millisecond)
		}
	}

	err = nil
	return
}

func InitStorerWithFetcher(fetcher Fetcher) (err error) {
	if fetcher == nil {
		return InitStorer()
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Create config storer with getter error. %v", r)
		}
	}()

	if configCache == nil {
		mutex.Lock()
		defer mutex.Unlock()
		if configCache == nil {
			opt := asynccache.Options{
				Fetcher:         fetcher,
				RefreshDuration: time.Duration((3000 + rand.Intn(2000))) * time.Millisecond,
			}
			configCache = asynccache.NewAsyncCache(opt)
		}
	}

	err = nil
	return
}

func Get(key string) (string, error) {
	val, err := configCache.Get(key)
	if val == nil {
		return "", err
	}
	if vstr, ok := val.(string); ok {
		if vstr == keyNotFound {
			return "", ErrEmpty
		}
		return vstr, err
	} else {
		return "", errors.New("value not string")
	}
}

func GetOrDefault(key, defaultVal string) (string, error) {
	val, err := Get(key)
	if IsKeyNonexist(err) {
		return defaultVal, nil
	}
	return val, err
}

func isEtcdKeyNonexist(err error) bool {
	return strings.Index(err.Error(), "100:") == 0
}

func IsKeyNonexist(err error) bool {
	return err == ErrEmpty
}
