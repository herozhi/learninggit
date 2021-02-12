# Metrics Package

公司 metrics server client 的 Go 实现。

- 出于性能考虑数据异步，maxPendingSize=1000 or emitInterval=200ms 两个条件满足之一才发送；
- 在 `metrics.NewDefaultMetricsClientV2` 时指定 nocheck=true 可以忽略烦人的 DefineXXX 调用；
- Value 支持类型 float32 float64 int int8 int16 int32 int64 uint8 uint16 uint32 uint64 time.Duration
    - 其中 time.Duration 将表示为 nanosecond
- v1 与 v2 的区别:
    - v1 在 new 时要求namespace，而 emit 时除了 metrics name 还要输入 prefix，容易让人误用。v2 统一了在New时指定前缀，后面只能emit 后缀；
    - v1 tags 用的 map 结构，在高并发下，遍历map性能消耗高。v2 换成了 slice 而且如果没tags时可以省略；
    - 当前 v1 底层实际用了 v2 的逻辑，tags map 转 slice 时为了内部优化做了sort，有额外消耗;
    - 新的项目应该都使用 v2;
    - 对于v2: 请保证没有重复tag name，否则metrics查询出来的结果是未定义的；
- 示例代码，详见 example/main.go
- 关于 metrics 查询的问题请到 tsdb 用户群发问，详见[研发公共群](https://bytedance.feishu.cn/space/doc/r9eZzWtluDmyp4G9QcnMBa)
