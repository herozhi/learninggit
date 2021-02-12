# GOPKG GORM

GOPKG GORM, 深度集成 ByteDance 专用功能, 例如 [bytedmysql](https://code.byted.org/gopkg/bytedmysql), 流量影子表, 压力测试, Logs...

## 初始化连接

```go
import gorm "code.byted.org/gopkg/gorm/v2"

db, err := gorm.POpen("bytedmysql", "XXXX_DSN")
// GOPKG GORM 集成了字节跳动的 MySQL Driver 实现 bytedmysql, 该 MySQL driver 由 https://code.byted.org/gopkg/bytedmysql 提供，他依赖了社区版本的 mysql driver，并加入一些 metrics 支持
// 字节跳动内部还有一套 mysql2 的 driver 实现，由 code.byted.org/gopkg/mysql-driver 提供，该 driver 是基于 https://github.com/go-sql-driver/mysql 较老版本的 fork，未来可能会逐渐使用 bytedmysql 替换

db, err := gorm.POpenWithConfig("bytedmysql", "XXXX_DSN", gorm.Config{
  SkipDefaultTransaction: true,
  NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
})

// DB 配置 (默认值)
db.SetConnMaxLifetime(time.Second * 300)
    .SetMaxIdleConns(100)
    .SetMaxOpenConns(50)

// 发起数据库请求，传入当前的 Context
err := db.NewRequest(ctx).Select(...).Where("a=?", aVal).Find(&data)

// db.NewRequest(ctx) 返回的对象为 *gorm.DB 对象，可以使用其 API 进行 DB 操作
```

**任何数据库请求需传入当前运行环境的 Context，该 Context 将应用在所有的后续 DB 操作及下面的测试支持判断中**

## 使用现有连接初始化

```go
import (
  "gorm.io/gorm"
  gormproxy "code.byted.org/gopkg/gorm/v2"
)

gormDB, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
gormProxy := gormproxy.POpenWithConn(gormDB)
```

## 读写分离

配置数据库读写分离

```go
db, err := gorm.POpen("bytedmysql", "write_DSN", "read1_DSN", "read2_DSN")

db, err := gorm.POpenWithConfig("bytedmysql", "write_DSN", gorm.Config{
  SkipDefaultTransaction: true,
  NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
}, "read1_DSN", "read2_DSN")
```

更复杂的读写分离请参考文档: [DBResolver](http://v2.gorm.io/docs/dbresolver.html)

```go
db, err := gorm.POpen("bytedmysql", "write_db1")

db.NewRequest(ctx).Use(dbresolver.Register(dbresolver.Config{
  Sources:  []gorm.Dialector{
              mysql.New(mysql.Config{DriverName: "bytedmysql", DSN: "db2"}),
            },
  Replicas: []gorm.Dialector{
              mysql.New(mysql.Config{DriverName: "bytedmysql", DSN: "db3"}),
              mysql.New(mysql.Config{DriverName: "bytedmysql", DSN: "db4"}),
            },
  Policy: dbresolver.RandomPolicy{},
}).Register(dbresolver.Config{
  Replicas: []gorm.Dialector{
              mysql.New(mysql.Config{DriverName: "bytedmysql", DSN: "db5"}),
            },
}, &User{}, &Address{}).Register(dbresolver.Config{
  Sources:  []gorm.Dialector{
              mysql.New(mysql.Config{DriverName: "bytedmysql", DSN: "db6"}),
              mysql.New(mysql.Config{DriverName: "bytedmysql", DSN: "db7"}),
            },
  Replicas: []gorm.Dialector{
              mysql.New(mysql.Config{DriverName: "bytedmysql", DSN: "db8"}),
            },
}, "orders", &Product{}, "secondary"))
```

## 设置 Logger

```go
db.WithLogger(logger)
```

其中 `logger` 的类型为: `code.byted.org/gopkg/logs.Logger`, 默认的 `logger` 为 `logs.DefaultLogger`

## 测试支持

### 支持测试流量影子表功能与测试降级

```go
// 标记测试流量，测试流量读写都会到影子表
ctx = context.WithValue(ctx, gorm.ContextStressKey, "test")

// 测试流量的读请求打到原表，写请求到影子表
ctx = context.WithValue(ctx, gorm.ContextSkipStressForRead, true)

// 拒绝测试流量，拒绝后所有的操作都不会发送命令到数据库. 可选值: gorm.SwitchOn, gorm.SwitchOff
// off 表示开启拒绝测试流量模式，默认将会拒绝所有测试流量
ctx = context.WithValue(ctx, gorm.ContextStressSwitch, gorm.SwitchOn)

// XXX 为 GORM 相关的 API
db = db.NewRequest(ctx).XXX
```

### 针对压力测试的额外配置

```go
// 当前请求需要把测试读流量打到原表（默认测试流量会读写影子表）
dbreq := db.NewRequestWithTestReadRequestToOrigin(ctx)
```

若配置，测试流量的渡请求会打到原表，等同于下面的代码：

```go
// 测试流量的读请求打到原表
ctx = context.WithValue(ctx, gorm.ContextSkipStressForRead, true)
```
