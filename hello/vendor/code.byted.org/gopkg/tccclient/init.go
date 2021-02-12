package tccclient

import (
	"os"

	"code.byted.org/gopkg/metrics"
	"code.byted.org/gopkg/net2"
	"code.byted.org/gopkg/tccclient/bconfig"
)

var (
	hostIP        = ""
	metricsClient *metrics.MetricsClientV2
)

const clientVersion = "v1.2.4"

func init() {
	if os.Getenv("MY_HOST_IP") != "" {
		hostIP = os.Getenv("MY_HOST_IP")
	} else if os.Getenv("HOST_IP_ADDR") != "" {
		hostIP = os.Getenv("HOST_IP_ADDR")
	}
	if hostIP == "" {
		hostIP = net2.GetLocalIPStr()
	}
	bconfig.SetHostIP(hostIP)

	metricsClient = metrics.NewDefaultMetricsClientV2("tcc", true)
	initV1()
}
