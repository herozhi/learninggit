package etcdproxy

import (
	"context"
	"errors"
	stdlog "log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"code.byted.org/gopkg/consul"
	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/etcdproxy/internal/singleflight"
)

const (
	timeout            = 100 * time.Millisecond
	minCacheTime       = 10 * time.Second
	defaultCacheTime   = 60 * time.Second
	defaultValueNotSet = "__etcdproxy_notset__"
)

type clusterContextKey struct{}

type getoptions struct {
	defaultv  string
	cluster   string
	addr      string
	cacheTime time.Duration
}

func (o *getoptions) HasDefault() bool {
	return o.defaultv != defaultValueNotSet
}

// GetOption represents option of get op
type GetOption func(o *getoptions)

// CacheTime sets value timeout to d
func CacheTime(d time.Duration) GetOption {
	if d < minCacheTime {
		d = minCacheTime
	}
	return func(o *getoptions) {
		o.cacheTime = d
	}
}

// DefaultValue sets default value of get on any cases
func DefaultValue(v string) GetOption {
	return func(o *getoptions) {
		o.defaultv = v
	}
}

// WithCluster sets cluster of get context
func WithCluster(cluster string) GetOption {
	return func(o *getoptions) {
		o.cluster = cluster
	}
}

// WithAddr sets addr for http request instead get from consul
func WithAddr(addr string) GetOption {
	return func(o *getoptions) {
		o.addr = addr
	}
}

var log = stdlog.New(os.Stderr, "", stdlog.LstdFlags)

// SetLogger sets default logger of etcdproxy
func SetLogger(logger *stdlog.Logger) {
	log = logger
}

type EtcdProxy struct {
	oo      getoptions
	sf      singleflight.Group
	cache   *Cache
	httpcli *http.Client

	cacheHit    int64
	cacheErr    int64
	proxyGet    int64
	proxyGetErr int64
}

// NewEtcdProxy creates instance of EtcdProxy
func NewEtcdProxy(opts ...GetOption) *EtcdProxy {
	dialer := net.Dialer{Timeout: timeout}
	oo := getoptions{defaultv: defaultValueNotSet, cluster: "default", cacheTime: defaultCacheTime}
	for _, op := range opts {
		op(&oo)
	}
	p := &EtcdProxy{oo: oo}
	p.cache = NewCache()
	p.httpcli = &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DisableCompression: true,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				if oo.addr != "" {
					return dialer.DialContext(ctx, network, oo.addr)
				}
				cluster := "default"
				if v := ctx.Value(clusterContextKey{}); v != nil {
					cluster = v.(string)
				}
				ee, err := lookupConsul("toutiao.etcd.proxy", cluster)
				if err != nil {
					return nil, err
				}
				return dialer.DialContext(ctx, network, ee.GetOne().Addr)
			},
		},
	}
	return p
}

type Stats struct {
	CacheHit    int64
	CacheMiss   int64
	CacheErr    int64
	ProxyGet    int64
	ProxyGetErr int64
}

func (p *EtcdProxy) Stats() Stats {
	var stats Stats
	stats.CacheHit = atomic.LoadInt64(&p.cacheHit)
	stats.CacheErr = atomic.LoadInt64(&p.cacheErr)
	stats.ProxyGet = atomic.LoadInt64(&p.proxyGet)
	stats.ProxyGetErr = atomic.LoadInt64(&p.proxyGetErr)
	return stats
}

// Get returns value specified by key
func (p *EtcdProxy) Get(key string, opts ...GetOption) (string, error) {
	oo := p.oo
	for _, op := range opts {
		op(&oo)
	}
	item := p.cache.Get(key)
	if item != nil {
		atomic.AddInt64(&p.cacheHit, 1)
		if item.Err != nil {
			atomic.AddInt64(&p.cacheErr, 1)
			if oo.HasDefault() {
				log.Printf("etcdproxy %q : %s (cached). using default: %q", key, item.Err, oo.defaultv)
				return oo.defaultv, nil
			}
		}
		return item.Value, item.Err
	}
	ret, err, _ := p.sf.Do(key+"@"+oo.cluster, func() (interface{}, error) {
		atomic.AddInt64(&p.proxyGet, 1)
		s, err := p.get(key, oo)
		if err != nil {
			atomic.AddInt64(&p.proxyGetErr, 1)
			p.cache.Set(key, Item{Err: err, Expires: time.Now().Add(oo.cacheTime / 2)})
		} else {
			p.cache.Set(key, Item{Value: s, Expires: time.Now().Add(oo.cacheTime)})
		}
		return s, err
	})
	s := ret.(string)
	if err != nil && oo.HasDefault() {
		log.Printf("etcdproxy %q : %s . using default: %q", key, err, oo.defaultv)
		return oo.defaultv, nil
	}
	return s, err
}

// Refresh refreshes cached value specified by key
func (p *EtcdProxy) Refresh(key string, opts ...GetOption) (string, error) {
	oo := p.oo
	for _, op := range opts {
		op(&oo)
	}
	ret, err, _ := p.sf.Do(key+"@"+oo.cluster, func() (interface{}, error) {
		atomic.AddInt64(&p.proxyGet, 1)
		s, err := p.get(key, oo)
		if err != nil {
			atomic.AddInt64(&p.proxyGetErr, 1)
			p.cache.Set(key, Item{Err: err, Expires: time.Now().Add(oo.cacheTime / 2)})
		} else {
			p.cache.Set(key, Item{Value: s, Expires: time.Now().Add(oo.cacheTime)})
		}
		return s, err
	})
	if err != nil {
		return "", err
	}
	return ret.(string), nil
}

func (p *EtcdProxy) get(key string, oo getoptions) (string, error) {
	if key == "" {
		return "", errors.New("key not specified")
	}
	uri := "http://toutiao.etcd.proxy/v2/keys/" + strings.TrimLeft(key, "/")
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return "", err
	}
	req = req.WithContext(context.WithValue(req.Context(), clusterContextKey{}, oo.cluster))
	resp, err := p.httpcli.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return decodeResponse(resp.Body)
}

var (
	mu    sync.Mutex
	proxy *EtcdProxy
)

// Get returns value specified by key
// shortcut of EtcdProxy.Get
func Get(key string, opts ...GetOption) (string, error) {
	mu.Lock()
	if proxy == nil {
		proxy = NewEtcdProxy()
	}
	mu.Unlock()
	return proxy.Get(key, opts...)
}

// Refresh refreshes cached value specified by key
// shortcut of EtcdProxy.Refresh
func Refresh(key string, opts ...GetOption) (string, error) {
	mu.Lock()
	if proxy == nil {
		proxy = NewEtcdProxy()
	}
	mu.Unlock()
	return proxy.Refresh(key, opts...)
}

func lookupConsul(psm, cluster string) (consul.Endpoints, error) {
	opts := []consul.LookupOptions{consul.WithCluster(cluster)}
	switch {
	case env.IsDualStack():
		opts = append(opts, consul.WithAddrFamily("dual-stack"), consul.WithUnique("v6"))
	case env.IsIPV6Only():
		opts = append(opts, consul.WithAddrFamily("v6"))
	default:
		opts = append(opts, consul.WithAddrFamily("v4"))
	}
	return consul.Lookup(psm, opts...)
}
