package tccclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"code.byted.org/gopkg/logs"
	"code.byted.org/gopkg/metrics"
	"code.byted.org/gopkg/tccclient/bconfig"
	"golang.org/x/sync/singleflight"
)

const (
	TCEGraySetting = "/var/tmp/tcc/service_settings/%s"

	GetTypeCache      = "cache"
	GetTypeStaleCache = "stale_cache" // refresh cache failed, read stale cache_data
	GetTypeServer     = "server"

	MaxDontCacheCount = 3
	firstGetTimeout   = 350 * time.Millisecond
	updateGetTimeout  = 200 * time.Millisecond

	refreshInterval = 10 * time.Second
	cacheTime       = refreshInterval + time.Second

	KeyCustomTimeout = "TCC_TIMEOUT"

	apiVersionV2 = "v2"
)

var (
	// ConfigNotFoundError service/confspace not found, or key not found
	ConfigNotFoundError = errors.New("config not found error")
	InternalError       = errors.New("tcc internal error")

	clientManager map[string]*ClientV2
	clientMu      sync.Mutex
)

// ClientV2 for TCC-V2
type ClientV2 struct {
	serviceName string
	confspace   string

	cache       *ConfigCache
	parserCache *ParserCache

	sf singleflight.Group

	bconfigClient *bconfig.BConfigClient
	getCount      uint64
	getSuccCount  uint64

	metaKey string
	dataKey string

	emiter *emiter

	listenInterval time.Duration
	listeners      map[string]*listener
	listening      bool
	listenerMu     sync.Mutex
}

func init() {
	clientManager = make(map[string]*ClientV2)
}

func NewClientV2(serviceName string, config *ConfigV2) (*ClientV2, error) {
	clientMu.Lock()
	defer clientMu.Unlock()
	clientKey := fmt.Sprintf("s:%s:c:%s", serviceName, config.Confspace)
	if client, ok := clientManager[clientKey]; ok && client != nil {
		return client, nil
	}
	if err := config.check(); err != nil {
		return nil, err
	}
	client := &ClientV2{}
	client.serviceName = serviceName
	client.confspace = config.Confspace
	client.cache = NewConfigCache(serviceName, config.Confspace, cacheTime)
	client.parserCache = NewParserCache()
	client.bconfigClient = bconfig.NewBConfigClient()
	client.getCount = 0
	client.getSuccCount = 0
	client.metaKey = fmt.Sprintf(KeyMetaFmt, client.serviceName)
	client.dataKey = fmt.Sprintf(KeyDataFmt, client.serviceName, client.confspace)
	client.emiter = newEmiter(client.serviceName, client.confspace)
	client.listeners = make(map[string]*listener)
	client.listenInterval = config.ListenInterval
	go client.refresh()
	clientManager[clientKey] = client
	return client, nil
}

func (c *ClientV2) Get(ctx context.Context, key string) (string, error) {
	value, _, err := c.getWithCache(ctx, key)
	return value, err
}

func (c *ClientV2) getWithCache(ctx context.Context, key string) (value string, versionCode string, err error) {
	getType := GetTypeCache
	defer func() { c.emiter.emit(err, key, getType) }()

	cacheData, cacheErr, expired := c.cache.Get()
	if !expired {
		return c.getValue(cacheData, cacheErr, key)
	}

	data, err := c.getAndCache(ctx, cacheData, cacheErr)
	getType = GetTypeServer
	if data == cacheData {
		getType = GetTypeStaleCache
	}
	return c.getValue(data, err, key)
}

func (c *ClientV2) getAndCache(ctx context.Context, cacheData *Data, cacheErr error) (*Data, error) {
	datai, err, _ := c.sf.Do(c.serviceName+"@"+c.confspace, func() (interface{}, error) {
		data, err := c.getFromServer(ctx, cacheData)
		if err == nil || err == ConfigNotFoundError {
			c.cache.Set(data, err)
			return data, err
		}
		if cacheData != nil || cacheErr == ConfigNotFoundError {
			c.cache.Set(cacheData, cacheErr)
			return cacheData, cacheErr
		}
		// if the first three times get data failed (not ConfigNotFoundError), do not cache the error
		if atomic.LoadUint64(&c.getCount) > MaxDontCacheCount || atomic.LoadUint64(&c.getSuccCount) > 0 {
			c.cache.Set(nil, err)
		}
		return nil, err
	})

	if err != nil {
		return nil, err
	}
	return datai.(*Data), nil
}

func (c *ClientV2) getFromServer(ctx context.Context, cacheData *Data) (data *Data, err error) {
	defer func() {
		atomic.AddUint64(&c.getCount, 1)
		if err == nil || err == ConfigNotFoundError {
			atomic.AddUint64(&c.getSuccCount, 1)
		}
	}()

	meta := &Meta{}
	if err := c.getAndMarshal(ctx, c.metaKey, meta); err != nil {
		return nil, err
	}
	versionCode, ok := meta.GetVersionCode(c.confspace)
	if !ok {
		return nil, ConfigNotFoundError
	}
	if cacheData != nil {
		if cacheData.VersionCode == versionCode {
			return &Data{
				Data:        cacheData.Data,
				GrayData:    cacheData.GrayData,
				VersionCode: cacheData.VersionCode,
				ModifyTime:  cacheData.ModifyTime,
				NeedGray:    c.needGray(ctx, cacheData),
			}, nil
		}
	}

	data = &Data{}
	if err := c.getAndMarshal(ctx, c.dataKey, data); err != nil {
		if err == ConfigNotFoundError {
			return nil, ConfigNotFoundError
		}
		return nil, err
	}
	if data.VersionCode != versionCode {
		logs.CtxWarn(ctx, "[tcc] config's version_code != meta's version_code: %v != %v", data.VersionCode, versionCode)
		ts := []metrics.T{
			metrics.T{Name: "service_name", Value: c.serviceName},
			metrics.T{Name: "confspace", Value: c.confspace},
		}
		metricsClient.EmitCounter("client.get_config.version_code.diff", 1, ts...)
	}
	data.NeedGray = c.needGray(ctx, data)
	return data, nil
}

func (c *ClientV2) needGray(ctx context.Context, data *Data) bool {
	grayData := data.GrayData
	if grayData == nil {
		return false
	}

	for _, ip := range grayData.GrayIPList {
		if ip == hostIP {
			return true
		}
	}

	tceGraySettings, err := c.readTCEGraySettings()
	if err != nil {
		if !os.IsNotExist(err) {
			logs.CtxWarn(ctx, "[tcc] read tce_gray_settings error: %v", err)
		}
		return false
	}
	for _, s := range tceGraySettings.GraySettings {
		if s.Confspace == c.confspace && s.GrayCode == grayData.GrayCode {
			return true
		}
	}
	return false
}

func (c *ClientV2) getAndMarshal(ctx context.Context, key string, model interface{}) error {
	getCtx, cancel := context.WithTimeout(context.Background(), c.getTimeout())
	defer cancel()

	value, err := c.bconfigClient.Get(getCtx, key)
	if err != nil {
		if bconfig.IsKeyNotFound(err) {
			return ConfigNotFoundError
		}
		logs.CtxWarn(ctx, "[tcc] get key: %v from bconfig error: %v", key, err)
		return err
	}
	err = json.Unmarshal([]byte(value), &model)
	if err != nil {
		logs.CtxWarn(ctx, "[tcc] unmarshal bconfig value error: %v", err)
		return err
	}
	return nil
}

func (c *ClientV2) getValue(data *Data, err error, key string) (string, string, error) {
	if err != nil {
		return "", "", err
	}
	if data == nil {
		return "", "", InternalError
	}

	if data.NeedGray && data.GrayData != nil {
		if value, ok := data.GrayData.Data[key]; ok {
			return value, data.VersionCode, nil
		}
	}
	if value, ok := data.Data[key]; ok {
		return value, data.VersionCode, nil
	}
	return "", "", ConfigNotFoundError
}

func (c *ClientV2) readTCEGraySettings() (*TCEGraySettings, error) {
	file, err := os.Open(fmt.Sprintf(TCEGraySetting, c.serviceName))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data := make([]byte, 2000)
	n, err := file.Read(data)
	if err != nil {
		return nil, err
	}
	tceGraySettings := &TCEGraySettings{}
	if err := json.Unmarshal(bytes.TrimSpace(data[:n]), tceGraySettings); err != nil {
		return nil, err
	}
	return tceGraySettings, nil
}

func (c *ClientV2) getTimeout() time.Duration {
	timeout := updateGetTimeout
	if atomic.LoadUint64(&c.getSuccCount) == 0 {
		timeout = firstGetTimeout
	}
	if customTimeout, err := strconv.Atoi(os.Getenv(KeyCustomTimeout)); err == nil {
		if (time.Duration(customTimeout) * time.Millisecond) > timeout {
			timeout = time.Duration(customTimeout) * time.Millisecond
		}
	}
	return timeout
}

func (c *ClientV2) refresh() {
	for {
		time.Sleep(refreshInterval)
		cacheData, cacheErr, lastAccessAt := c.cache.GetWithAccessAt()
		if cacheData == nil {
			continue
		}
		if now().Unix()-lastAccessAt > int64(float64(refreshInterval/time.Second)*1.5) {
			continue
		}
		c.getAndCache(context.Background(), cacheData, cacheErr) // nolint: errcheck
	}
}
