package tccclient

import (
	"context"
	"fmt"
	"time"

	"code.byted.org/gopkg/logs"
)

const minListenInterval = 60 * time.Second

// Callback for listener
type Callback func(value string, err error)

type listenOptions struct {
	curValue *string
}

// ListenOption represents option of listener
type ListenOption func(o *listenOptions)

// WithCurrentValue set current value of key
func WithCurrentValue(value string) ListenOption {
	return func(o *listenOptions) {
		o.curValue = &value
	}
}

type listener struct {
	key             string
	callback        Callback
	lastVersionCode string
	lastValue       string
	lastErr         error
}

func (l *listener) update(value, versionCode string, err error) {
	if versionCode == l.lastVersionCode && err == l.lastErr {
		return
	}
	if value == l.lastValue && err == l.lastErr {
		// version_code updated, but value not updated
		l.lastVersionCode = versionCode
		return
	}
	defer func() {
		if r := recover(); r != nil {
			logs.Errorf("[tcc] listener callback panic, key: %s, %v", l.key, r)
		}
	}()
	l.callback(value, err)
	l.lastVersionCode = versionCode
	l.lastValue = value
	l.lastErr = err
}

// AddListener add listener of key, if key's value updated, callback will be called
func (c *ClientV2) AddListener(key string, callback Callback, opts ...ListenOption) error {
	listenOps := listenOptions{}
	for _, op := range opts {
		op(&listenOps)
	}

	listener := listener{
		key:      key,
		callback: callback,
	}
	if listenOps.curValue == nil {
		listener.update(c.getWithCache(context.Background(), key))
	} else {
		listener.lastValue = *listenOps.curValue
	}

	c.listenerMu.Lock()
	defer c.listenerMu.Unlock()
	if _, ok := c.listeners[key]; ok {
		return fmt.Errorf("[tcc] listener already exist, key: %s", key)
	}
	c.listeners[key] = &listener
	if !c.listening {
		go c.listen()
		c.listening = true
	}
	return nil
}

func (c *ClientV2) listen() {
	for {
		time.Sleep(c.listenInterval)
		listeners := c.getListeners()
		for key := range listeners {
			listeners[key].update(c.getWithCache(context.Background(), key))
		}
	}
}

func (c *ClientV2) getListeners() map[string]*listener {
	c.listenerMu.Lock()
	defer c.listenerMu.Unlock()
	listeners := make(map[string]*listener, len(c.listeners))
	for key := range c.listeners {
		listeners[key] = c.listeners[key]
	}
	return listeners
}
