package gorm

import (
	"context"
	"errors"
	"time"

	_ "code.byted.org/gopkg/bytedmysql"
	"code.byted.org/gopkg/logs"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"gorm.io/plugin/dbresolver"
)

const (
	StressTestTablePostfix   = "_stress"
	ContextStressKey         = "K_STRESS"
	ContextSkipStressForRead = "K_SKIP_STRESS"
	ContextStressSwitch      = "K_STRESS_SWITCH"

	SwitchOn  = "on"
	SwitchOff = "off"
)

var stressTestTablePostfixBytes = []byte(StressTestTablePostfix)

type DBProxy struct {
	db         *gorm.DB
	dbResolver *dbresolver.DBResolver
}

type Config gorm.Config

func POpenWithConn(db *gorm.DB) (dbProxy *DBProxy, err error) {
	dbProxy = &DBProxy{db: db}

	if dbProxy.db.Logger == logger.Default {
		dbProxy.db.Logger = defaultLogger
	}

	dbProxy.SetConnMaxLifetime(time.Second * 300).SetMaxIdleConns(100).SetMaxOpenConns(50)

	// RegisterStressTestCallbacks
	dbProxy.RegisterStressTestCallbacks()

	return dbProxy, nil
}

func POpenWithConfig(dialect string, dsn string, cfg Config, replicasDSN ...string) (dbProxy *DBProxy, err error) {
	dbProxy = &DBProxy{}

	if cfg.Logger == nil {
		cfg.Logger = defaultLogger
	}

	switch dialect {
	case "mysql", "mysql2", "bytedmysql", "bytetx_mysql":
		gormConfig := gorm.Config(cfg)
		dbProxy.db, err = gorm.Open(mysql.New(mysql.Config{
			DriverName: dialect,
			DSN:        dsn,
		}), &gormConfig)
	default:
		err = errors.New("only support mysql driver")
	}

	if err != nil {
		return nil, err
	}

	if len(replicasDSN) > 0 {
		var dialectors []gorm.Dialector
		for _, replicaDSN := range replicasDSN {
			dialectors = append(dialectors, mysql.New(mysql.Config{DriverName: dialect, DSN: replicaDSN}))
		}

		dbResolver := dbresolver.Register(dbresolver.Config{
			Replicas: dialectors,
		}).SetConnMaxLifetime(time.Second * 300).SetMaxIdleConns(100).SetMaxOpenConns(50)

		dbProxy.dbResolver = dbResolver
		dbProxy.db.Use(dbResolver)
	} else {
		dbProxy.SetConnMaxLifetime(time.Second * 300).SetMaxIdleConns(100).SetMaxOpenConns(50)
	}

	// RegisterStressTestCallbacks
	dbProxy.RegisterStressTestCallbacks()

	return dbProxy, nil
}

func POpen(dialect string, dsn string, replicasDSN ...string) (dbProxy *DBProxy, err error) {
	return POpenWithConfig(dialect, dsn, Config{}, replicasDSN...)
}

func (p *DBProxy) RegisterStressTestCallbacks() {
	// register stress test callbacks
	//   read operations
	p.db.Callback().Query().Before("gorm:query").Register("bytedance:stress_test", StressTestCallback(true))
	p.db.Callback().Row().Before("gorm:row").Register("bytedance:stress_test", StressTestCallback(true))
	//   write operations
	p.db.Callback().Create().Before("gorm:before_create").Register("bytedance:stress_test", StressTestCallback(false))
	p.db.Callback().Delete().Before("gorm:before_delete").Register("bytedance:stress_test", StressTestCallback(false))
	p.db.Callback().Update().Before("gorm:before_update").Register("bytedance:stress_test", StressTestCallback(false))
	p.db.Callback().Raw().Before("gorm:raw").Register("bytedance:stress_test", StressTestCallback(false))
}

func (p *DBProxy) UseDBResolver(dbResolver *dbresolver.DBResolver) {
	p.dbResolver = dbResolver
	p.db.Use(dbResolver)
}

func (p *DBProxy) SetConnMaxLifetime(dur time.Duration) *DBProxy {
	if p.dbResolver != nil {
		p.dbResolver.SetConnMaxLifetime(dur)
	} else if sqlDB, err := p.db.DB(); err == nil {
		sqlDB.SetConnMaxLifetime(dur)
	}
	return p
}

func (p *DBProxy) SetMaxIdleConns(n int) *DBProxy {
	if p.dbResolver != nil {
		p.dbResolver.SetMaxIdleConns(n)
	} else if sqlDB, err := p.db.DB(); err == nil {
		sqlDB.SetMaxIdleConns(n)
	}
	return p
}

func (p *DBProxy) SetMaxOpenConns(n int) *DBProxy {
	if p.dbResolver != nil {
		p.dbResolver.SetMaxOpenConns(n)
	} else if sqlDB, err := p.db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(n)
	}
	return p
}

func (p *DBProxy) SingularTable(enable bool) *DBProxy {
	if namingStrategy, ok := p.db.NamingStrategy.(*schema.NamingStrategy); ok {
		namingStrategy.SingularTable = true
	}
	return p
}

func (p *DBProxy) LogMode(enable bool) *DBProxy {
	if enable {
		p.db.Logger = p.db.Logger.LogMode(logger.Info)
	} else {
		p.db.Logger = p.db.Logger.LogMode(logger.Error)
	}
	return p
}

func (p *DBProxy) WithLogger(l *logs.Logger) *DBProxy {
	p.db.Logger = Logger{LogLevel: logger.Info, Logger: l}
	return p
}

func (p *DBProxy) NewRequestWithTestReadRequestToOrigin(ctx context.Context) *gorm.DB {
	return p.NewRequest(context.WithValue(ctx, ContextSkipStressForRead, true))
}

func (p *DBProxy) NewRequest(ctx context.Context) *gorm.DB {
	// Turn on DryRun mode for testing requests and ContextStressSwitch equals off
	if isTestRequest(ctx) {
		if switchVal, ok := ctx.Value(ContextStressSwitch).(string); !ok || switchVal == SwitchOff {
			return p.db.Session(&gorm.Session{Context: ctx, DryRun: true})
		}
	}

	return p.db.Session(&gorm.Session{Context: ctx})
}
