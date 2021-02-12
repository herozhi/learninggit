package gorm

import (
	"code.byted.org/gopkg/etcd_util"
	"code.byted.org/gopkg/logs"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type DBProxy struct {
	db              *DB
	dbName          string
	dbSwitchKey     string
	openDynamicConf bool
}

func (p *DBProxy) initConfig() (err error) {
	defer func() {
		if r := recover(); r != nil {
			logs.Errorf("Init config error, %v", r)
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	etcdutil.GetWithDefault("/kite/stressbot/request/switch/global", "off")
	if len(p.dbName) > 0 && p.dbName != "unknown" {
		p.dbSwitchKey = fmt.Sprintf("/kite/stressbot/db/%s/switch", p.dbName)
		etcdutil.GetWithDefault(p.dbSwitchKey, "off")
	}

	err = nil
	return
}

func GetDatabaseName(dialect string, dsn string) (string, error) {
	switch dialect {
	case "mysql", "mysql2":
		stopPos := strings.LastIndexByte(dsn, '?')
		if stopPos == -1 {
			stopPos = len(dsn)
		}
		startPos := strings.LastIndexByte(string(dsn[0:stopPos]), '/')
		if startPos == -1 || startPos == len(dsn)-1 {
			return "", fmt.Errorf("get db name wrong from: %s for %s", dsn, dialect)
		}
		return string(dsn[startPos+1 : stopPos]), nil
	case "postgres":
		pos := strings.Index(dsn, "dbname=")
		if pos == -1 || pos+7 >= len(dsn) {
			return "", fmt.Errorf("get db name wrong from: %s for %s", dsn, dialect)
		}
		endPos := strings.IndexByte(string(dsn[pos+7:]), ' ')
		if endPos == -1 {
			return string(dsn[pos+7:]), nil
		}
		return string(dsn[pos+7 : pos+7+endPos]), nil
	case "mssql":
		pos := strings.LastIndexByte(dsn, '?')
		if pos == -1 || pos == len(dsn)-1 {
			return "", fmt.Errorf("get db name wrong from: %s for %s", dsn, dialect)
		}
		queryStr := string(dsn[pos+1:])
		params := strings.Split(queryStr, "&")
		for _, param := range params {
			if strings.HasPrefix(param, "database=") && len(param) > 9 {
				return string(param[9:]), nil
			}
		}
		return "", fmt.Errorf("get db name wrong from: %s for %s", dsn, dialect)
	case "sqlite", "sqlite3":
		if strings.HasPrefix(dsn, "/tmp/") && len(dsn) > 5 {
			return string(dsn[5:]), nil
		}
		return "", fmt.Errorf("get db name wrong from: %s for %s", dsn, dialect)
	}
	return "", fmt.Errorf("not supported dialect. get from: %s for %s", dsn, dialect)
}

func POpen(dialect string, args ...interface{}) (*DBProxy, error) {
	return openWithConf(dialect, false, args...)
}

func POpenWithDynamicConf(dialect string, args ...interface{}) (*DBProxy, error) {
	return openWithConf(dialect, true, args...)
}

func openWithConf(dialect string, dynamicConf bool, args ...interface{}) (*DBProxy, error) {
	if len(args) == 0 {
		return nil, errors.New("miss dsn")
	}
	dbName := "unknown"
	if dsn, ok := args[0].(string); ok {
		dbnameTmp, err := GetDatabaseName(dialect, dsn)
		if err == nil {
			dbName = dbnameTmp
		}
	}

	gdb, err := Open(dialect, args...)
	if err != nil {
		return nil, err
	}

	p := &DBProxy{
		db:              gdb,
		dbName:          dbName,
		openDynamicConf: dynamicConf,
	}
	err = p.initConfig()
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *DBProxy) SetConnMaxLifetime(dur time.Duration) *DBProxy {
	p.db.DB().SetConnMaxLifetime(dur)
	return p
}

func (p *DBProxy) SetMaxIdleConns(n int) *DBProxy {
	p.db.DB().SetMaxIdleConns(n)
	return p
}

func (p *DBProxy) SetMaxOpenConns(n int) *DBProxy {
	p.db.DB().SetMaxOpenConns(n)
	return p
}

func (p *DBProxy) SingularTable(enable bool) *DBProxy {
	p.db.SingularTable(enable)
	return p
}

func (p *DBProxy) LogMode(enable bool) *DBProxy {
	p.db.LogMode(enable)
	return p
}

func (p *DBProxy) WithLogger(l *logs.Logger) *DBProxy {
	p.db.LogMode(true)
	p.db.SetExternalBaseLogger(l)
	return p
}

func (p *DBProxy) NewRequestWithTestReadRequestToOrigin(ctx context.Context) *DB {
	ctx = context.WithValue(ctx, ContextSkipStressForRead, true)
	ctx = p.stressSwitchContext(ctx)
	return p.db.Context(ctx)
}

func (p *DBProxy) NewRequest(ctx context.Context) *DB {
	ctx = p.stressSwitchContext(ctx)
	return p.db.Context(ctx)
}

func (p *DBProxy) stressSwitchContext(ctx context.Context) context.Context {
	if !p.openDynamicConf {
		return ctx
	}

	globalSwitch := etcdutil.GetWithDefault("/kite/stressbot/request/switch/global", "off")
	if globalSwitch == "off" {
		ctx = context.WithValue(ctx, ContextStressSwitch, SwitchOff)
		return ctx
	} else if globalSwitch != "on" {
		ctx = context.WithValue(ctx, ContextStressSwitch, SwitchOff)
		return ctx
	}

	dbSwitch := etcdutil.GetWithDefault(p.dbSwitchKey, "off")
	if dbSwitch == "off" {
		ctx = context.WithValue(ctx, ContextStressSwitch, SwitchOff)
		return ctx
	} else if dbSwitch != "on" {
		ctx = context.WithValue(ctx, ContextStressSwitch, SwitchOff)
		return ctx
	}
	return ctx
}
