# GORM

Mirror of github.com/jinzhu/gorm

[GORM 相关群/文档/最佳实践](https://bytedance.feishu.cn/docs/doccnOzRftbJXP4cGrqxsoAkCAV)

[![GORM depended count](https://badge.byted.org/go/depended-count/byte-gitlab/gopkg/gorm "GORM depended count")](https://code.byted.org/gopkg/gorm)
[![GORM latest version](https://badge.byted.org/go/version/byte-gitlab/gopkg/gorm "GORM latest version")](https://code.byted.org/gopkg/gorm)

## GORM V2 升级

升级到 GORM V2 请参考 https://code.byted.org/gopkg/gorm/tree/master/v2

## 所有数据库请求必须传入Context
强烈建议使用新接口
```go
import "code.byted.org/gopkg/gorm"

db := gorm.POpen("mysql2", "XXXX_DSN")

//db配置
db.SetConnMaxLifetime(lifeTime)
    .SetMaxIdleConns(100)
    .SetMaxOpenConns(50)

//发起数据库请求，传入当前的Context
err := db.NewRequest(ctx).Select(...).Where("a=?", aVal).Find(&data)
```
任何数据库请求必须传入当前运行环境的Context。

## 设置logger
```go
db.WithLogger(logger)
```
其中logger类型为:code.byted.org/gopkg/logs.Logger。

## [测试支持] 支持测试流量影子表功能与测试降级
```go
//标记测试流量，测试流量读写都会到影子表
ctx = context.WithValue(gorm.ContextStressKey, "test")


//测试流量的读请求打到原表
ctx = context.WithValue(gorm.ContextSkipStressForRead, true)

//拒绝测试流量，所有操作拒绝. value: on/off，默认开启.
//use constant: gorm.SwitchOn, gorm.SwitchOff
ctx = context.WithValue(gorm.ContextStressSwitcher, gorm.SwitchOn)

db = db.NewRequest(ctx).XXX
```
__注意：__ 以上操作不支持Raw SQL，即直接传入SQL执行。

## 支持动态配置
```go
db := gorm.POpenWithDynamicConf("mysql2", "your DSN")
```
### 压测动态开关
在etcd的kite v3集群配置。
1. /kite/stressbot/request/switch/global 压测全局开关  on/off
2. /kite/stressbot/db/#{databaseName}/switch 数据库测试流量开关 on/off

拒绝后，该请求会失败，调用db.Error返回error。

## 针对压力测试的额外配置
```go
//当前请求需要把测试读流量打到原表（默认测试流量会读写影子表）
dbreq := db.NewRequestWithTestReadRequestToOrigin(ctx)
```
若配置，测试流量的渡请求会打到原表，等同于下面的代码：
```go
//测试流量的读请求打到原表
ctx = context.WithValue(gorm.ContextSkipStressForRead, true)
```
