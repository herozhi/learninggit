package kitevent

import "time"

// KitEvent 表示kitc内部一些自动化触发的事件
type KitEvent struct {
	Name   string
	Time   time.Time
	Detail string
	Extra  map[string]interface{} // Read only
}
