package clientv1

import (
	"net/http"
	"time"

	"code.byted.org/gopkg/env"
	"code.byted.org/middleware/framework_version_collector_client"
	"code.byted.org/middleware/framework_version_collector_client/model"
)

const uri = "/v1/collect"

func CollectWithRetry(name, version string, extra interface{}, protocol string, protocolExtra interface{}, retryTimes int, interval time.Duration, logger client.Logger) {
	for i := 0; i <= retryTimes; i++ {
		resp, err := Collect(name, version, extra, protocol, protocolExtra, logger)
		if err == nil {
			if resp != nil {
				resp.Body.Close()
			}
			break
		}
		logger.Warn("collect version info failed: %v, retry after %v", err, interval.String())
		time.Sleep(interval)
	}
}

func Collect(name, version string, extra interface{}, protocol string, protocolExtra interface{}, logger client.Logger) (*http.Response, error) {
	if !env.IsProduct() && !env.IsBoe() {
		logger.Info("not prod environment, skip collect version info")
		return nil, nil
	}
	psm := env.PSM()
	cluster := env.Cluster()
	idc := env.IDC()
	stage := env.Stage()
	if psm == env.PSMUnknown || cluster == "" || idc == env.UnknownIDC || stage == env.UnknownStage {
		logger.Info("not prod environment, skip collect version info")
		return nil, nil
	}
	v := &model.CollectData{
		PSM:           psm,
		Cluster:       cluster,
		IDC:           idc,
		Env:           stage,
		Name:          name,
		Version:       version,
		Extra:         extra,
		Protocol:      protocol,
		ProtocolExtra: protocolExtra,
	}
	url := client.Domain + uri
	return client.PostJson(url, v, logger)
}

func CollectRaw(psm, cluster, idc, stage, name, version string, extra interface{}, protocol string, protocolExtra interface{}, logger client.Logger) (*http.Response, error) {
	if psm == "" || psm == env.PSMUnknown || cluster == "" || idc == "" || idc == env.UnknownIDC || stage == "" || stage == env.UnknownStage {
		logger.Info("not prod environment, skip collect version info")
		return nil, nil
	}
	v := &model.CollectData{
		PSM:           psm,
		Cluster:       cluster,
		IDC:           idc,
		Env:           stage,
		Name:          name,
		Version:       version,
		Extra:         extra,
		Protocol:      protocol,
		ProtocolExtra: protocolExtra,
	}
	url := client.Domain + uri
	return client.PostJson(url, v, logger)
}
