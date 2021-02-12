package kitc

import (
	"code.byted.org/kite/kitutil"
	"code.byted.org/kite/kitutil/kitevent"
)

// ATTENTION: 如果新增定义事件, 需要考虑在下方 installEventListener 里同时增加该事件监听, 否则该新增事件默认被忽略
const (
	REMOTE_CONFIG_CHANGE      string = "remote_config_change"
	REMOTE_CONFIG_IDC_REMOVE  string = "remote_config_idc_remove"
	SERVICE_CB_CHANGE         string = "service_circuitbreaker_change"
	INSTANCE_CB_CHANGE        string = "bad_ip_circuitbreaker_change"
	USER_ERR_CB_CHANGE        string = "user_error_circuitbreaker_change"
	DOWNSTREAM_INS_CHANGE     string = "downstream_ins_change"
	SERVICE_ADDRESS_CHANGE    string = "service_address_change"
	SERVICE_DISCOVERY_SUCCESS string = "service_discovery_success"
)

// installeventListener establishes a standard handler for kitc which pushes event-in-interests to the given queue.
func installEventListener(eventBus kitevent.EventBus, inspector kitutil.Inspector) (eventQueues []kitevent.Queue) {
	standardEvents := []string{REMOTE_CONFIG_CHANGE, REMOTE_CONFIG_IDC_REMOVE, SERVICE_CB_CHANGE, INSTANCE_CB_CHANGE, USER_ERR_CB_CHANGE,
		DOWNSTREAM_INS_CHANGE}
	for _, event := range standardEvents {
		eq := kitevent.NewQueue(10)
		eventBus.Watch(event, func(event *kitevent.KitEvent) {
			eq.Push(event)
		})
		eventQueues = append(eventQueues, eq)
	}

	inspector.Register(func(data map[string]interface{}) {
		for i, event := range standardEvents {
			data[event] = eventQueues[i].Dump()
		}
	})
	return
}
