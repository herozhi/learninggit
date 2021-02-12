# Bytedance Redis client for Golang

## Branch from
- [original redis v6](https://github.com/go-redis/redis)
- commit:  ddbd81ea6c66514fe1f857af890276fcac7921ce
- time:    Sun Sep 3 10:31:40 2017
- 后续修改可直接从本工程commit获取

## 初衷
原生v6对connection的pool，pop和push都在slice的尾部进行，这种设计对当前的一些使用场景存在负载不均的问题：
- 隔段时间有并发访问，会建立大量的新连接
- 其他时间，串行使用client，导致每次都选择了pool尾部的connection

## 修改为
```
diff --git a/internal/pool/pool.go b/internal/pool/pool.go
--- a/internal/pool/pool.go
+++ b/internal/pool/pool.go
@@ -203,9 +203,9 @@ func (p *ConnPool) popFree() *Conn {
      return nil
        }

- idx := len(p.freeConns) - 1
- cn := p.freeConns[idx]
- p.freeConns = p.freeConns[:idx]
+ idx := len(p.freeConns)
+ cn := p.freeConns[0]
+ p.freeConns = p.freeConns[1:idx]
return cn
}
```
## 影响
- IdleTimeout基本不可用，因为理论上所有的连接会被轮寻用到，导致可能很低的qps，会维持一个较大size的pool（不超过conf中的PoolSize）
- 性能降低，因为freeConns push尾部，pop头部，到时会有频繁的内存申请和释放，测试每个请求在此处会增加时延1μs
