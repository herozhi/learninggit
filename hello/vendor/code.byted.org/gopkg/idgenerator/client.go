package idgenerator

import (
	"time"

	"code.byted.org/gopkg/logs"
)

const (
	defaultTimeout time.Duration = 200 * time.Millisecond
	minTimeout     time.Duration = 100 * time.Millisecond
)

type options struct {
	timeout time.Duration
}

type Option func(o *options)

// WithTimeout set timeout of http request
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) {
		if timeout < minTimeout {
			logs.Warn("idgenerator timeout: %v < minTimoue: %v, set timeout = %v", timeout, minTimeout, minTimeout)
			timeout = minTimeout
		}
		o.timeout = timeout
	}
}

func newIdGeneratorClient(namespace string, countspace string, need64Bit bool, opts ...Option) *IdGeneratorClient {
	options := &options{timeout: defaultTimeout}
	for _, opt := range opts {
		opt(options)
	}

	client := IdGeneratorClient{
		namespace:  namespace,
		countspace: countspace,
		need64Bit:  need64Bit,
		timeout:    options.timeout,
	}
	return &client
}

//New64BitIdGeneratorClient ...
func New64BitIdGeneratorClient(namespace string, countspace string, opts ...Option) *IdGeneratorClient {
	return newIdGeneratorClient(namespace, countspace, true, opts...)
}

// New64BitIdGeneratorWrapper ...
func New64BitIdGeneratorWrapper(namespace string, countspace string, count int, opts ...Option) *IdGeneratorWrapper {
	client := New64BitIdGeneratorClient(namespace, countspace, opts...)
	wrapper := newIdGeneratorWrapper(client, count)
	return wrapper
}

//New52BitIdGeneratorClient ...
func New52BitIdGeneratorClient(namespace string, countspace string, opts ...Option) *IdGeneratorClient {
	return newIdGeneratorClient(namespace, countspace, false, opts...)
}

// New52BitIdGeneratorWrapper ...
func New52BitIdGeneratorWrapper(namespace string, countspace string, count int, opts ...Option) *IdGeneratorWrapper {
	client := New52BitIdGeneratorClient(namespace, countspace, opts...)
	wrapper := newIdGeneratorWrapper(client, count)
	return wrapper
}
