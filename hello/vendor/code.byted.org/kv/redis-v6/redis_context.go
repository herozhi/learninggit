// +build go1.7

package redis

import (
	"context"

	"code.byted.org/kv/redis-v6/pkg/pool"
)

type baseClient struct {
	connPool pool.Pooler
	opt      *Options

	process             func(Cmder) error
	onClose             func() error                     // hook called when client is closed
	getConnAddition     func() (*pool.Conn, bool, error) // get connection
	releaseConnAddition func(*pool.Conn, error) bool     // release connection

	ctx context.Context
}

func (c *Client) Context() context.Context {
	if c.ctx != nil {
		return c.ctx
	}
	return context.Background()
}

func (c *Client) WithContext(ctx context.Context) *Client {
	if ctx == nil {
		panic("nil context")
	}
	c2 := c.copy()
	c2.ctx = ctx
	return c2
}
