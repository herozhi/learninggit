# ctxvalues

![](https://codebase-view.bytedance.net/api/invoke/badge?app_id=gopkg/ctxvalues)
![](https://coverage-badge.bytedance.net/api/invoke/badge?app_id=gopkg/ctxvalues)
[![](https://img.shields.io/badge/godoc-reference-blue)](https://codebase.byted.org/godoc/code.byted.org/gopkg/ctxvalues)

从 `context.Context` 中获取字节跳动（ByteDance）预定义值的包

## 如何安装

```shell script
go get code.byted.org/gopkg/ctxvalues
```

## 如何使用

### 导入

```go
import "code.byted.org/gopkg/ctxvalues"
```

### logid

- 获取 logid

```go
logid, _ := ctxvalues.LogID(ctx)
```

- 设置 logid

```go
ctx = ctxvalues.SetLogID(ctx, logid)
```

## 变更记录 ChangeLog

- v0.0.1 2020.08.20
  - 添加 LogID，获取 logid
