package ginex

import (
	"strconv"
	"time"

	"code.byted.org/gin/ginex/internal"
	internal_util "code.byted.org/gin/ginex/internal/util"
	"code.byted.org/gopkg/logs"
	clientv1 "code.byted.org/middleware/framework_version_collector_client/v1"
)

type ProtocolExtra struct {
	PSM      string    `json:"psm"`
	Mappings []Mapping `json:"mappings"`
}
type Mapping struct {
	Method       string `json:"method"`
	PathPattern  string `json:"path_pattern"`
	FunctionName string `json:"function_name"`
}

func reportMetainfo(extra map[string]string, protocolExtra interface{}) {
	infos := make(map[string]string)
	infos["psm"] = PSM()
	infos["cluster"] = internal_util.LocalCluster()
	infos["language"] = "go"
	infos["framework"] = "ginex"
	infos["framework_version"] = internal.VERSION
	infos["protocol"] = "http"
	infos["ip"] = LocalIP()
	infos["port"] = strconv.Itoa(appConfig.ServicePort)
	infos["debug_port"] = strconv.Itoa(appConfig.DebugPort)

	if extra != nil {
		for k, v := range extra {
			infos[k] = v
		}
	}
	go collectVersionInfo(infos, protocolExtra)
}

func collectVersionInfo(metaInfo map[string]string, protocolExtra interface{}) {
	// 最多尝试两次上报
	clientv1.CollectWithRetry(metaInfo["framework"], metaInfo["framework_version"], nil, metaInfo["protocol"], protocolExtra, 2, 10*time.Second, logs.DefaultLogger())
}
