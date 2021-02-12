package kitc

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"code.byted.org/gopkg/logs"
	"code.byted.org/kite/endpoint"
	clientv1 "code.byted.org/middleware/framework_version_collector_client/v1"
)

const (
	Version         = "v3.10.6"
	Name            = "kitc"
	Protocol        = "thrift"
	ProtocolVersion = "0.9.2"
)

var ServiceMeshMode bool
var ServiceMeshEgressAddr string

func init() {
	if os.Getenv("SERVICE_MESH_EGRESS_ADDR") != "" {
		ServiceMeshMode = true
		ServiceMeshEgressAddr = os.Getenv("SERVICE_MESH_EGRESS_ADDR")
		fmt.Fprintf(os.Stdout, "KITC: open service mesh egress mode, proxy address: %s\n", ServiceMeshEgressAddr)
	}
}

func OpenMeshMode(meshEgressAddr string) {
	ServiceMeshMode = true
	ServiceMeshEgressAddr = meshEgressAddr
}

// Client is a interface that every generate code should implement this
type Client interface {
	New(kc *KitcClient) Caller
}

// Caller is a interface for client do soem RPC calling
type Caller interface {
	Call(name string, request interface{}) (endpoint.EndPoint, endpoint.KitcCallRequest)
}

// client just is an instance have some methods, client will contains connections
var clients = make(map[string]Client)

// Register ...
func Register(name string, client Client) {
	if client == nil {
		panic("KITC: Register client is nil")
	}
	if _, dup := clients[name]; dup {
		panic(fmt.Sprintf("KITC: Register dup client %s", name))
	}
	clients[name] = client
}

var kitcClients = make(map[string]*KitcClient) // [name:code location]*client
var kitcClientsLock sync.RWMutex

func registerKitcClient(client *KitcClient) {
	_, file, line, ok := runtime.Caller(3)
	var key string
	if !ok {
		key = client.Name()
	} else {
		key = fmt.Sprintf("%s:%s:%d", client.Name(), file, line)
	}
	kitcClientsLock.Lock()
	defer kitcClientsLock.Unlock()
	kitcClients[key] = client

	go collectVersionInfo(client)
}

func collectVersionInfo(c *KitcClient) {
	extra := struct {
		ServiceMesh bool `json:"service_mesh"`
	}{
		ServiceMesh: c.opts.MeshMode,
	}
	protocolExtra := struct {
		Version string `json:"version"`
	}{
		Version: ProtocolVersion,
	}

	clientv1.CollectWithRetry(Name, Version, extra, Protocol, protocolExtra, 2, 10*time.Second, logs.DefaultLogger())
}

// AllKitcClients returns all registered clients
func AllKitcClients() map[string]*KitcClient {
	clients := make(map[string]*KitcClient)
	kitcClientsLock.RLock()
	defer kitcClientsLock.RUnlock()
	for k, c := range kitcClients {
		clients[k] = c
	}
	return clients
}

func DebugInfo() interface{} {
	clients := AllKitcClients()
	results := make(map[string]interface{})

	for name, client := range clients {
		psm := client.Name()
		sub := make(map[string]interface{})
		events := client.inspector.Collect()
		sub["psm"] = psm
		sub["events"] = events
		sub["options"] = client.Options().Marshal()
		sub["remote_configs"] = client.RemoteConfigs()
		results[name] = sub
	}
	return results
}

// IDLs .
var IDLs = make(map[string]map[string]string) // map[filename]content

// SetIDL called by code generated by kitool to define method
func SetIDL(service, filename, content string) {
	if _, ok := IDLs[service]; !ok {
		IDLs[service] = make(map[string]string)
	}
	IDLs[service][filename] = content
}
