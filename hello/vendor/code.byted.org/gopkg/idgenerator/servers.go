package idgenerator

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"code.byted.org/gopkg/consul"

	"code.byted.org/gopkg/logs"
)

var (
	idgenPSM = "toutiao.webarch.idgenerator_proxy"

	mu           sync.RWMutex
	idgenServers []string
)

func getServers() []string {
	mu.RLock()
	if len(idgenServers) != 0 {
		servers := append(make([]string, 0, len(idgenServers)), idgenServers...)
		mu.RUnlock()
		return servers
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()
	if len(idgenServers) == 0 {
		newServers, err := getServersFromConsul()
		if err != nil {
			logs.Error("idgenerator get servers from consul failed: %v", err)
		} else {
			idgenServers = newServers
		}
	}
	servers := append(make([]string, 0, len(idgenServers)), idgenServers...)
	return servers
}

func doUpdateServer() {
	for {
		newServers, err := getServersFromConsul()
		if err != nil {
			logs.Error("idgenerator get servers from consul failed: %v", err)
		} else {
			if len(idgenServers) == 0 {
				logs.Info("idgenerator servers: %v", newServers)
			}
			mu.Lock()
			idgenServers = newServers
			mu.Unlock()
		}

		time.Sleep(60 * time.Second)
	}
}

func getServersFromConsul() ([]string, error) {
	endpoints, err := consul.LookupName(idgenPSM)
	if err != nil {
		return nil, err
	}

	servers := []string{}
	for _, ep := range endpoints {
		servers = append(servers, fmt.Sprintf("http://%s/", strings.TrimSpace(ep.Addr)))
	}

	if len(servers) == 0 {
		return nil, errors.New("consul: empty servers")
	}
	return servers, nil
}
