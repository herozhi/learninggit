package kite

import (
	"fmt"
	"strings"
	"time"

	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/logs"
	"code.byted.org/gopkg/thrift"
	clientv1 "code.byted.org/middleware/framework_version_collector_client/v1"
)

func (p *RpcServer) startDaemonRoutines() {

	if EnableMetrics {
		GoStatMetrics()
	}

	startDebugServer()

	go func() { // print connection metrics
		for range time.Tick(time.Second * 5) {
			m := fmt.Sprintf("service.thrift.%s.conns", ServiceName)
			metricsClient.EmitStore(m, p.overloader.ConnNow())
		}
	}()

	go func() { // update overload config period
		if ServiceMeshMode {
			return
		}

		for range time.Tick(time.Second * 5) {
			connLim, err := p.remoteConfiger.GetConnLimit()
			if err != nil {
				logs.Warnf("KITE: get remote connection limit err: %s\n", err.Error())
			} else if connLim != p.overloader.ConnLimit() {
				logs.Infof("KITE: update remote connection limit to: %d, old: %d\n", connLim,
					p.overloader.ConnLimit())
				p.overloader.UpdateConnLimit(connLim)
			}

			qpsLim, err := p.remoteConfiger.GetQPSLimit()
			if err != nil {
				logs.Warnf("KITE: get remote qps limit err: %s\n", err.Error())
			} else if qpsLim != p.overloader.QPSLimit() {
				logs.Infof("KITE: update remote qps limit to: %d, old: %d\n", qpsLim,
					p.overloader.QPSLimit())
				p.overloader.UpdateQPSLimit(qpsLim)
			}

			endpointQPSLimit, err := p.remoteConfiger.getEndpointQPSLimit()
			if err != nil {
				logs.Warnf("KITE: get remote endpoint qps limit err: %s\n", err.Error())
			} else {
				p.overloader.UpdateEndpointQPSLimit(endpointQPSLimit)
			}
		}
	}()

	// collect version info, only do once
	go collectVersionInfo(p.Metainfo())
}

func collectVersionInfo(metaInfo map[string]string) {
	extra := struct {
		EnableTracing bool `json:"enable_tracing"`
		ServiceMesh   bool `json:"service_mesh"`
	}{
		EnableTracing: EnableTracing,
		ServiceMesh:   ServiceMeshMode,
	}

	protocolExtra := struct {
		Version      string `json:"version"`
		InProtocol   string `json:"in_protocol"`
		InTransport  string `json:"in_transport"`
		OutProtocol  string `json:"out_protocol"`
		OutTransport string `json:"out_transport"`
	}{
		Version:      metaInfo["thrift_version"],
		InProtocol:   metaInfo["thrift_in_protocol"],
		InTransport:  metaInfo["thrift_in_transport"],
		OutProtocol:  metaInfo["thrift_out_protocol"],
		OutTransport: metaInfo["thrift_out_transport"],
	}

	clientv1.CollectWithRetry(metaInfo["framework"], metaInfo["framework_version"], extra, metaInfo["protocol"], protocolExtra, 2, 10*time.Second, defaultLogger)
}

// Metainfo .
func (p *RpcServer) Metainfo() map[string]string {
	getThriftProtocolType := func(protocolFactory thrift.TProtocolFactory) string {
		switch protocolFactory.(type) {
		case *thrift.TBinaryProtocolFactory:
			return "binary"
		case *thrift.TCompactProtocolFactory:
			return "compact"
		}
		return "other"
	}
	getThriftTransportType := func(transportFactory thrift.TTransportFactory) string {
		switch transportFactory.(type) {
		case *thrift.TBufferedTransportFactory:
			return "buffered"
		}
		return "other"
	}

	infos := make(map[string]string)
	infos["psm"] = ServiceName
	infos["cluster"] = ServiceCluster
	infos["language"] = "go"
	infos["framework"] = "kite"
	infos["framework_version"] = Version
	infos["protocol"] = "thrift"
	infos["ip"] = env.HostIP()
	infos["port"] = ServicePort
	debugPort := DebugServerPort
	if p := strings.Index(debugPort, ":"); p != -1 {
		debugPort = debugPort[p+1:]
	}
	infos["debug_port"] = debugPort
	infos["thrift_in_protocol"] = getThriftProtocolType(p.protocolFactory)
	infos["thrift_in_transport"] = getThriftTransportType(p.transportFactory)
	infos["thrift_out_protocol"] = getThriftProtocolType(p.protocolFactory)
	infos["thrift_out_transport"] = getThriftTransportType(p.transportFactory)
	infos["thrift_version"] = "0.9.2"
	return infos
}
