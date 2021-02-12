// +build !go1.7

package redis

import (
	"code.byted.org/kv/redis-v6/pkg/pool"
)

type baseClient struct {
	connPool pool.Pooler
	opt      *Options

	process             func(Cmder) error
	onClose             func() error                     // hook called when client is closed
	getConnAddition     func() (*pool.Conn, bool, error) // get connection
	releaseConnAddition func(*pool.Conn, error) bool     // release connection
}
