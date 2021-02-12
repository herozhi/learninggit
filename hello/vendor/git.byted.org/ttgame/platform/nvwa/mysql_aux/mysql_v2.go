package mysql_aux

import (
	"errors"
	"fmt"
	"time"

	"code.byted.org/gopkg/gorm/v2"
	"code.byted.org/gopkg/logs"

	_ "code.byted.org/gopkg/mysql-driver"
	"git.byted.org/ttgame/platform/nvwa/conf"
)

func NewClientV2WithConf() (mdbWrite, mdbRead *gorm.DBProxy, err error) {
	return NewClientV2WithConfPrefixKey("mysql")
}

func NewClientV2WithConfPrefixKey(prefixKey string) (mdbWrite, mdbRead *gorm.DBProxy, err error) {
	if prefixKey == "" {
		logs.Fatalf("prefix can not be empty")
		err = errors.New("prefix can not be empty")
		return
	}
	loc := getConf(prefixKey, "location")
	if loc == "" {
		loc = "Local"
	}
	if getConf(prefixKey, "write_psm") == "" && getConf(prefixKey, "read_psm") == "" {
		logs.Fatalf("read and write psm can not empty both")
		err = errors.New("read and write psm can not empty both")
		return
	}
	if getConf(prefixKey, "write_psm") != "" {
		dsnWrite := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=%s&timeout=%s&readTimeout=%s&writeTimeout=%s",
			getConf(prefixKey, "write_user_name"),
			getConf(prefixKey, "write_password"),
			getConf(prefixKey, "write_psm"),
			getConf(prefixKey, "db_name"),
			loc,
			getConf(prefixKey, "timeout"),
			getConf(prefixKey, "read_timeout"),
			getConf(prefixKey, "write_timeout"))
		mdbWrite, err = gorm.POpen("mysql2", dsnWrite)
		if err != nil {
			logs.Fatalf("new mysql client error, dsn=%v, err=%v", dsnWrite, err)
			return
		}
		maxIdle := conf.MustGetInt(makeKey(prefixKey, "write_max_idle"))
		if maxIdle != 0 {
			mdbWrite.SetMaxIdleConns(maxIdle)
		}
		maxOpen := conf.MustGetInt(makeKey(prefixKey, "max_open"))
		if maxOpen != 0 {
			mdbWrite.SetMaxOpenConns(maxOpen)
		}
		maxLifeTime := conf.MustGetDuration(makeKey(prefixKey, "max_life_time"))
		if maxLifeTime != time.Duration(0) {
			mdbWrite.SetConnMaxLifetime(maxLifeTime)
		}
	}

	if getConf(prefixKey, "read_psm") != "" {
		dsnRead := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=%s&timeout=%s&readTimeout=%s",
			getConf(prefixKey, "read_user_name"),
			getConf(prefixKey, "read_password"),
			getConf(prefixKey, "read_psm"),
			getConf(prefixKey, "db_name"),
			loc,
			getConf(prefixKey, "timeout"),
			getConf(prefixKey, "read_timeout"))
		mdbRead, err = gorm.POpen("mysql2", dsnRead)
		if err != nil {
			logs.Fatalf("new mysql client error, dsn=%v, err=%v", dsnRead, err)
			return
		}
		maxIdle := conf.MustGetInt(makeKey(prefixKey, "read_max_idle"))
		if maxIdle != 0 {
			mdbRead.SetMaxIdleConns(maxIdle)
		}
		maxOpen := conf.MustGetInt(makeKey(prefixKey, "max_open"))
		if maxOpen != 0 {
			mdbRead.SetMaxOpenConns(maxOpen)
		}
		maxLifeTime := conf.MustGetDuration(makeKey(prefixKey, "max_life_time"))
		if maxLifeTime != time.Duration(0) {
			mdbRead.SetConnMaxLifetime(maxLifeTime)
		}
	}
	return
}
