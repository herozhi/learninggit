package ssconf

import (
	"errors"
	"fmt"
	"strings"
)

var (
	InvalidKeyErr     = errors.New("conf invalid key")
	InvalidValueErr   = errors.New("conf invalid value")
	InvalidClusterErr = errors.New("conf invalid cluster")
)

func ExtraToList(line string, sep string) []string {
	return line2List(line, sep)
}

func line2List(line string, sep string) []string {
	serverlist := make([]string, 0)
	servers := strings.Split(line, ",")
	for _, server := range servers {
		serverlist = append(serverlist, strings.TrimSpace(server))
	}
	return serverlist
}

func GetTotalConfigure(confFile string) (map[string]string, error) {
	rets, err := LoadSsConfFile(confFile)
	if err != nil {
		return nil, err
	}
	return rets, nil
}

func GetConfigureOnDemand(confFile, consulKey string) (map[string]string, error) {
	rets, err := LoadSsConfFileOnDemand(confFile, consulKey)
	if err != nil {
		return nil, err
	}
	return rets, nil
}

func GetServersFromCache(rets map[string]string, key string) ([]string, error) {
	line, status := rets[key]
	if status == false {
		return nil, InvalidKeyErr
	}
	if len(line) == 0 {
		return nil, InvalidValueErr
	}
	serverList := line2List(line, ",")
	return serverList, nil
}

func GetServerPortsFromCache(rets map[string]string, keyHost, keyPort string) ([]string, error) {
	lineHost, status := rets[keyHost]
	if status == false {
		return nil, InvalidKeyErr
	}
	linePort, status := rets[keyPort]
	if status == false {
		return nil, InvalidKeyErr
	}
	hostList := line2List(lineHost, ",")
	portList := line2List(linePort, ",")
	servers := make([]string, 0)
	for i := 0; i < len(hostList); i++ {
		server := fmt.Sprintf("%s:%s", strings.TrimSpace(hostList[i]), strings.TrimSpace(portList[i]))
		servers = append(servers, server)
	}
	return servers, nil
}

func GetServerList(confFile, key string) ([]string, error) {
	rets, err := GetTotalConfigure(confFile)
	if err != nil {
		return nil, err
	}
	return GetServersFromCache(rets, key)
}

func GetServerListOnDemand(confFile, consulKey string) ([]string, error) {
	rets, err := GetConfigureOnDemand(confFile, consulKey)
	if err != nil {
		return nil, err
	}
	return GetServersFromCache(rets, consulKey)
}

func ParseClusterAndServerList(confFile, category string) (map[string][]string, error) {
	rets, err := LoadSsConfFile(confFile)
	if err != nil {
		return nil, err
	}

	cluster_to_servers := make(map[string][]string)
	for key, value := range rets {
		if len(value) == 0 {
			continue
		}
		if category == "memcache" {
			if strings.HasSuffix(key, "use_proxy") {
				continue
			}
		} else if category == "springdb" {
			if !strings.HasSuffix(key, "servers") {
				continue
			}
		} else if category == "table" {
			if !strings.HasSuffix(key, "tables") {
				continue
			}
		}
		serverlist := line2List(value, ",")
		cluster_to_servers[key] = serverlist
	}
	if len(cluster_to_servers) == 0 {
		return nil, InvalidClusterErr
	}
	return cluster_to_servers, nil
}

func ParseMcServerListAndProxy(confFile string) (map[string][]string, map[string]bool, error) {
	rets, err := LoadSsConfFile(confFile)
	if err != nil {
		return nil, nil, err
	}
	cluster_to_servers := make(map[string][]string)
	use_proxy_conf := make(map[string]bool)
	proxy_suffix := "_use_proxy"
	for key, value := range rets {
		if len(value) == 0 {
			continue
		}
		if strings.HasSuffix(key, proxy_suffix) {
			cluster_name := key[:len(key)-len(proxy_suffix)]
			use_proxy_conf[cluster_name] = true
		} else {
			serverlist := line2List(value, ",")
			cluster_to_servers[key] = serverlist
		}
	}
	return cluster_to_servers, use_proxy_conf, nil
}
