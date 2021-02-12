package bytedmysql

import (
	"context"
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
)

// 实现了 Driver 的各种接口
type Connection struct {
	WrappedConn driver.Conn
	Cfg         *mysql.Config
	DBPSM       string
}

func (c *Connection) Begin() (driver.Tx, error) {
	now := time.Now()
	tx, err := c.WrappedConn.Begin()
	cost := time.Since(now).Nanoseconds() / int64(time.Microsecond)
	doMetrics("begin", "begin", c.DBPSM, cost, err)
	return tx, err
}

func (c *Connection) Close() (err error) {
	return c.WrappedConn.Close()
}

func (c *Connection) Prepare(query string) (driver.Stmt, error) {
	now := time.Now()
	stmt, err := c.WrappedConn.Prepare(query)
	cost := time.Since(now).Nanoseconds() / int64(time.Microsecond)
	doMetrics("prepare", query, c.DBPSM, cost, err)
	return stmt, err
}

func (c *Connection) Exec(query string, args []driver.Value) (driver.Result, error) {
	if execer, ok := c.WrappedConn.(driver.Execer); ok {
		now := time.Now()
		query = interpolate(query)
		r, err := execer.Exec(query, args)
		cost := time.Since(now).Nanoseconds() / int64(time.Microsecond)
		doMetrics("exec", query, c.DBPSM, cost, err)
		return r, err
	}
	// 不可能出现这种情况的, sql 已经进行 database/sql assert 了
	return nil, fmt.Errorf("[bytedmysql] wrappedconn is not Execer")
}

func (c *Connection) Query(query string, args []driver.Value) (driver.Rows, error) {
	if queryer, ok := c.WrappedConn.(driver.Queryer); ok {
		now := time.Now()
		query = interpolate(query)
		r, err := queryer.Query(query, args)
		cost := time.Since(now).Nanoseconds() / int64(time.Microsecond)
		doMetrics("query", query, c.DBPSM, cost, err)
		return r, err
	}
	return nil, fmt.Errorf("[bytedmysql] wrappedconn is not Queryer")
}

func (c *Connection) Ping(ctx context.Context) error {
	if pinger, ok := c.WrappedConn.(driver.Pinger); ok {
		now := time.Now()
		err := pinger.Ping(ctx)
		cost := time.Since(now).Nanoseconds() / int64(time.Microsecond)
		doMetrics("ping", "ping", c.DBPSM, cost, err)
		return err
	}
	return fmt.Errorf("[bytedmysql] warppedconn is not Pinger")
}

func (c *Connection) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if ciCtx, ok := c.WrappedConn.(driver.ConnBeginTx); ok {
		now := time.Now()
		tx, err := ciCtx.BeginTx(ctx, opts)
		cost := time.Since(now).Nanoseconds() / int64(time.Microsecond)
		doMetrics("begin", "begin", c.DBPSM, cost, err)
		return tx, err
	}
	return nil, fmt.Errorf("[bytedmysql] WrappedConn is not ConnBeginTx")
}

func (c *Connection) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if queryerCtx, ok := c.WrappedConn.(driver.QueryerContext); ok {
		now := time.Now()
		query = interpolate(query)
		r, err := queryerCtx.QueryContext(ctx, query, args)
		cost := time.Since(now).Nanoseconds() / int64(time.Microsecond)
		doMetrics("query", query, c.DBPSM, cost, err)
		return r, err
	}
	return nil, fmt.Errorf("[bytedmysql] WrappedConn is not QueryerContext")
}

func (c *Connection) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if execerCtx, ok := c.WrappedConn.(driver.ExecerContext); ok {
		now := time.Now()
		query = interpolate(query)
		r, err := execerCtx.ExecContext(ctx, query, args)
		cost := time.Since(now).Nanoseconds() / int64(time.Microsecond)
		doMetrics("exec", query, c.DBPSM, cost, err)
		return r, err
	}
	return nil, fmt.Errorf("[bytedmysql] WrappedConn is not ExecerContext")
}

func (c *Connection) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if ciCtx, is := c.WrappedConn.(driver.ConnPrepareContext); is {
		now := time.Now()
		query = interpolate(query)
		stmt, err := ciCtx.PrepareContext(ctx, query)
		cost := time.Since(now).Nanoseconds() / int64(time.Microsecond)
		doMetrics("prepare", query, c.DBPSM, cost, err)
		return stmt, err
	}
	return nil, fmt.Errorf("[bytedmysql] WrappedConn is not PrepareContext")
}

func (c *Connection) CheckNamedValue(nv *driver.NamedValue) (err error) {
	if checker, ok := c.WrappedConn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(nv)
	}
	return fmt.Errorf("[bytedmysql] WrappedConn is not NamedValueChecker")
}

func (c *Connection) ResetSession(ctx context.Context) error {
	if resetter, ok := c.WrappedConn.(driver.SessionResetter); ok {
		return resetter.ResetSession(ctx)
	}
	return fmt.Errorf("[bytedmysql] WrappedConn is not SessionResetter")
}
