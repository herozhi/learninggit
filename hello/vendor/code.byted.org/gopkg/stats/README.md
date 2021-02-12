# Go在线监控包

用于将当运行的Golang服务的一些监控状态输出到Metrics中，用于监控。

### Metrics变量

go.{ReportName}.{item}

其中item列表如下：

* heap
* stack
* gcPause
* numGos
* numGcs
