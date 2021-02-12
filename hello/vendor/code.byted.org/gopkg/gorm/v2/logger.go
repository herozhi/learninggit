package gorm

import (
	"context"
	"time"

	"code.byted.org/gopkg/logs"
	"gorm.io/gorm/logger"
)

var defaultLogger = Logger{Logger: logs.DefaultLogger()}

type Logger struct {
	logger.LogLevel
	*logs.Logger
}

func (l Logger) LogMode(level logger.LogLevel) logger.Interface {
	l.LogLevel = level
	return l
}

func (l Logger) Info(ctx context.Context, msg string, data ...interface{}) {
	l.Logger.CtxInfo(ctx, "GORM LOG "+msg, data...)
}

func (l Logger) Warn(ctx context.Context, msg string, data ...interface{}) {
	l.Logger.CtxWarn(ctx, "GORM LOG "+msg, data...)
}

func (l Logger) Error(ctx context.Context, msg string, data ...interface{}) {
	l.Logger.CtxError(ctx, "GORM LOG "+msg, data...)
}

func (l Logger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel > 0 {
		cost := float64(time.Since(begin).Nanoseconds()/1e4) / 100.0
		switch {
		case err != nil && l.LogLevel >= logger.Error:
			sql, _ := fc()
			logs.CtxError(ctx, "GORM LOG %s, Error: %s", sql, err.Error())
		case l.LogLevel >= logger.Info:
			sql, _ /* affected rows */ := fc()
			logs.CtxInfo(ctx, "GORM LOG SQL:%s Cost:%.2fms", sql, cost)
		}
	}
}
