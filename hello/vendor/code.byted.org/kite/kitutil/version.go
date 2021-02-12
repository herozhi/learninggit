package kitutil

import (
	"time"

	"code.byted.org/gopkg/logs"
	clientv1 "code.byted.org/middleware/framework_version_collector_client/v1"
)

const (
	Name    = "kitutil"
	Version = "v3.7.17"
)

func init() {
	go clientv1.CollectWithRetry(Name, Version, nil, "", nil, 3, 5*time.Second, logs.DefaultLogger())
}
