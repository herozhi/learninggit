package mysql_aux

import (
	"errors"
	"fmt"
	"time"

	"code.byted.org/gopkg/gorm"
	"code.byted.org/gopkg/logs"
	_ "code.byted.org/gopkg/mysql-driver"
	"git.byted.org/ttgame/platform/nvwa/conf"
)

/*
mysql_read_user_name,mysql_read_password,mysql_write_user_name,mysql_write_password为开发环境专属
mysql_db_name sdk_server_test
mysql_read_psm 10.2.198.97:3306
mysql_read_user_name root #开发环境用
mysql_read_password 123456 #开发环境用
mysql_write_psm 10.2.198.97:3306
mysql_write_user_name root #开发环境用
mysql_write_password 123456 #开发环境
mysql_timeout 1000ms
mysql_read_timeout 2.0s
mysql_write_timeout 5.0s
mysql_location
mysql_read_max_idle 10
mysql_write_max_idle 5
mysql_max_open 50
mysql_max_life_time 60s
*/

func NewClientWithConf() (mdbWrite, mdbRead *gorm.DBProxy, err error) {
	return NewClientWithConfPrefixKey("mysql")
}

func makeKey(prefixKey, confKey string) string {
	return fmt.Sprintf("%s_%s", prefixKey, confKey)
}

func getConf(prefixKey, confKey string) string {
	return conf.GetConf(makeKey(prefixKey, confKey))
}

func NewClientWithConfPrefixKey(prefixKey string) (mdbWrite, mdbRead *gorm.DBProxy, err error) {
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
