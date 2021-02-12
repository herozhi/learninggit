package ratelimit

import (
	"sync/atomic"
	"time"
)

/*
 * 之前所使用的令牌桶都是支持了各种复杂功能，所以相对而言比较复杂
 * 实际上很多功能在对于QPS限制是用不到的，可以精简掉
 * 由于QPSLimiter会在每个请求访问的时候都调用到
 * 所以以精简部分对QPS无用的功能为代价，设计出一个专注于QPS限制的、高性能的令牌桶
 * 没有采用类似tokenbucket的方法，在每次获取令牌的时候去更新
 * 直接起了一个goroutine去更新令牌数量
 * 只要设置出一个合理的interval，就能兼顾性能和精确程度
 * 设置的interval越大，误差越小；设置的interval越小，误差越大
 *
 * 根据benchmark以及pprof，性能比tokenbucket好非常多
 *
 * benchmark结果如下：
 * Bucket-12                 150ns ± 2%
 * QpsLimiter-12            0.32ns ± 0%
 * BucketConcurrent-12      2.70µs ± 4%
 * QPSLimiterConcurrent-12  0.35ns ± 0%
 */

// 只用来做QPS的Limiter
type QPSLimiter struct {
	limit    int32 // 应该没有需求承受大于21亿qps的吧……
	tokens   int32
	interval time.Duration
	once     int32 // 每一个interval补充多少
	ticker   *time.Ticker
}

func NewQPSLimiter(interval time.Duration, limit int32) *QPSLimiter {
	once := limit / int32(time.Second/interval)
	if once == 0 {
		once = 1
	}
	l := &QPSLimiter{
		limit:    limit,
		tokens:   limit,
		interval: interval,
		once:     once,
		ticker:   time.NewTicker(interval),
	}
	go l.startTicker()
	return l
}

// UpdateLimiter 方法不是并发安全的，不要并发去调用
func (l *QPSLimiter) UpdateLimiter(interval time.Duration, limit int32) {
	l.limit = limit
	once := limit / int32(time.Second/interval)
	l.once = once
	if interval != l.interval {
		l.interval = interval
		l.stopTicker()
		l.ticker = time.NewTicker(interval)
		go l.startTicker()
	}
}

// Take 方法是线程安全的
func (l *QPSLimiter) Take() bool {
	if atomic.LoadInt32(&l.tokens) <= 0 {
		return false
	}
	return atomic.AddInt32(&l.tokens, -1) >= 0
}

func (l *QPSLimiter) QPSLimit() int32 {
	return atomic.LoadInt32(&l.limit)
}

func (l *QPSLimiter) Interval() time.Duration {
	return l.interval
}

func (l *QPSLimiter) startTicker() {
	ch := l.ticker.C
	for range ch {
		l.updateToken()
	}
}

func (l *QPSLimiter) stopTicker() {
	l.ticker.Stop()
}

// 这里允许一些误差，直接Store，可以换来更好的性能，也解决了大并发情况之下CAS不上的问题 by chengguozhu
func (l *QPSLimiter) updateToken() {
	var v int32
	v = atomic.LoadInt32(&l.tokens)
	if v < 0 {
		v = l.once
	} else if v+l.once > l.limit {
		v = l.limit
	} else {
		v = v + l.once
	}
	atomic.StoreInt32(&l.tokens, v)
}
