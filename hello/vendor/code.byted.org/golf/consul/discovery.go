package consul

import (
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var (
	configed   bool = false
	isPerfTest bool = false
	perfPrefix      = ""
	whitelist       = make(map[string]struct{})
)

const tcePerfTestWhitelistType = "consul"

func configPerfTest() {
	if configed {
		return
	}
	configed = true
	if _, ok := os.LookupEnv("TCE_PERF_TEST"); ok {
		isPerfTest = true
		perfPrefix = os.Getenv("TCE_PERF_PREFIX")
		if perfPrefix == "" {
			perfPrefix = "tce_perf_test_a3b30390ca0c_"
		}
		for _, part := range strings.Split(os.Getenv("TCE_PERF_WHITELIST"), "&") {
			if strings.HasPrefix(part, tcePerfTestWhitelistType) {
				key := part[len(tcePerfTestWhitelistType)+1:]
				whitelist[key] = struct{}{}
			}
		}
	}
}

type Endpoint struct {
	Host string
	Port int
	Tags map[string]string
}

type ServiceDiscovery struct {
	Client    *Client
	Dc        string
	Domain    string
	AgentHost string
	AgentPort int
}

const serviceSuffix = "service"
const weakConsistency = "stale"
const strongConsistency = "consistent"

var (
	defaultAgentHost = "127.0.0.1"
	defaultAgentPort = 2280
)

func addPerfPrefix(name string) string {
	configPerfTest()
	_, ok := whitelist[name]
	if !isPerfTest || ok || strings.HasPrefix(name, perfPrefix) {
		return name
	}

	return perfPrefix + name
}

func getConfig(client *Client) (string, string, error) {
	info, err := client.Agent().Self()
	if err != nil {
		return "unknown", "unknown", err
	}
	dc := info["Config"]["Datacenter"].(string)
	domain := info["Config"]["Domain"].(string)
	return dc, domain, nil
}

func stripDomain(s string) string {
	return strings.Trim(s, ".")
}

func loadServiceNode(entry *ServiceEntry) *Endpoint {
	return &Endpoint{
		entry.Node.Address,
		entry.Service.Port,
		LoadTags(entry.Service.Tags),
	}
}

func LoadTags(serviceTags []string) map[string]string {
	tags := make(map[string]string)
	for _, item := range serviceTags {
		cols := strings.Split(item, ":")
		// case 0: ['k0:v0'] => {'k0': 'v0'}
		// case 1: ['k0:v0', 'master'] => {'master': True}
		if len(cols) > 1 {
			tags[cols[0]] = cols[1]
		} else {
			tags[cols[0]] = "True"
		}
	}
	return tags
}

func dumpTags(tags map[string]string) []string {
	result := make([]string, len(tags))
	i := 0
	for key, value := range tags {
		result[i] = fmt.Sprintf("%s:%s", key, value)
		i++
	}
	return result
}

func NewServiceDiscovery() (*ServiceDiscovery, error) {
	return NewSpecifiedServiceDiscovery("", 0)
}

func NewSpecifiedServiceDiscovery(agentHost string, agentPort int) (*ServiceDiscovery, error) {
	config := DefaultConfig()
	if agentHost == "" {
		if agentHost = os.Getenv("CONSUL_HTTP_HOST"); agentHost != "" {
			defaultAgentHost = agentHost
		} else {
			agentHost = defaultAgentHost
		}
	} else {
		defaultAgentHost = agentHost
	}
	if agentPort == 0 {
		if agentPortStr := os.Getenv("CONSUL_HTTP_PORT"); agentPortStr != "" {
			agentPort, _ = strconv.Atoi(agentPortStr)
			defaultAgentPort = agentPort
		} else {
			agentPort = defaultAgentPort
		}
	} else {
		defaultAgentPort = agentPort
	}

	config.Address = fmt.Sprintf("%s:%d", agentHost, agentPort)
	client, err := NewClient(config)
	if err != nil {
		return nil, err
	}
	dc, domain, err := getConfig(client)
	if err != nil {
		return nil, err
	}
	return &ServiceDiscovery{
		client, dc, domain, agentHost, agentPort,
	}, nil
}

func NewServiceDiscoveryWithoutTimeout() (*ServiceDiscovery, error) {
	return NewSpecifiedServiceDiscoveryWithoutTimeout("", 0)
}

func NewSpecifiedServiceDiscoveryWithoutTimeout(agentHost string, agentPort int) (*ServiceDiscovery, error) {
	config := DefaultConfig()
	if agentHost == "" {
		if agentHost = os.Getenv("CONSUL_HTTP_HOST"); agentHost != "" {
			defaultAgentHost = agentHost
		} else {
			agentHost = defaultAgentHost
		}
	} else {
		defaultAgentHost = agentHost
	}
	if agentPort == 0 {
		if agentPortStr := os.Getenv("CONSUL_HTTP_PORT"); agentPortStr != "" {
			agentPort, _ = strconv.Atoi(agentPortStr)
			defaultAgentPort = agentPort
		} else {
			agentPort = defaultAgentPort
		}
	} else {
		defaultAgentPort = agentPort
	}

	config.HttpClient = &http.Client{}
	config.Address = fmt.Sprintf("%s:%d", agentHost, agentPort)
	client, err := NewClient(config)
	if err != nil {
		return nil, err
	}
	dc, domain, err := getConfig(client)
	if err != nil {
		return nil, err
	}
	return &ServiceDiscovery{
		client, dc, domain, agentHost, agentPort,
	}, nil
}

// canonical service name, e.g. "comment" => "comment.hy.byted.org."
// can be further used as DNS domain name
func (sd *ServiceDiscovery) CanonicalName(service string) string {
	if strings.HasSuffix(service, sd.Domain) {
		return service
	}
	return strings.Join([]string{service, serviceSuffix, sd.Dc, sd.Domain}, ".")
}

// convert canonical service name to (service, dc)
func (sd *ServiceDiscovery) ResolveName(name string) (string, string) {
	name = stripDomain(name)
	if strings.HasSuffix(name, sd.Domain) {
		name = name[0 : len(name)-len(sd.Domain)-1]
	}
	separator := fmt.Sprintf(".%s", serviceSuffix)
	var service string
	var dc string
	if strings.Contains(name, separator) {
		cols := strings.Split(name, separator)
		service, dc = cols[0], stripDomain(cols[1])
		if len(dc) < 1 {
			dc = sd.Dc
		}
	} else {
		service, dc = name, sd.Dc
	}
	return service, dc
}

/*
 * WARN: use weak consistency causes stale read
 * unless explicitly specific otherwise (values: 'default', 'consistent' or 'stale')
 * 'default': R/W goes to leader without quorum verification of leadership
 * 'consistency': R/W goes to leader with quorum verification of leadership (extra RTT)
 * 'stale': R goes to any server, W goes to leader
 */
func (sd *ServiceDiscovery) lookupInternal(name, consistency string) ([]*Endpoint, error) {
	name = addPerfPrefix(name)
	service, dc := sd.ResolveName(name)
	opts := &QueryOptions{
		dc,
		consistency == weakConsistency,
		consistency == strongConsistency,
		0,
		0,
		"",
	}
	entries, _, err := sd.Client.Health().Service(service, "", true, opts)
	if err != nil {
		return nil, err
	}
	result := make([]*Endpoint, len(entries))
	i := 0
	for _, entry := range entries {
		result[i] = loadServiceNode(entry)
		i++
	}
	return result, nil
}

func (sd *ServiceDiscovery) Lookup(name string) (endpoints []*Endpoint, err error) {
	endpoints, err = sd.lookupInternal(name, weakConsistency)
	return
}

func (sd *ServiceDiscovery) LookupConsistent(name string) (endpoints []*Endpoint, err error) {
	endpoints, err = sd.lookupInternal(name, strongConsistency)
	if err != nil {
		// if fail, fallback to WEAK_CONSISTENCY
		// this can occur during leader failure, hence try out follower nodes
		endpoints, err = sd.lookupInternal(name, weakConsistency)
	}
	return
}

func (sd *ServiceDiscovery) Announce(name string, port int, tags map[string]string, ttl int) (int, error) {
	name = addPerfPrefix(name)
	_, dc := sd.ResolveName(name)
	if dc != sd.Dc {
		return -1, fmt.Errorf("Datacenter mismatch: %s <-> %s", dc, sd.Dc)
	}
	serviceId := fmt.Sprintf("%s-%d", name, port)
	checkId := fmt.Sprintf("service:%s", serviceId)
	err := sd.Client.Agent().PassTTL(checkId, "")
	if err != nil {
		serviceTags := dumpTags(tags)
		checkTTL := fmt.Sprintf("%ds", ttl)
		check := &AgentServiceCheck{TTL: checkTTL, Status: "passing"}
		reg := &AgentServiceRegistration{
			ID:    serviceId,
			Name:  name,
			Tags:  serviceTags,
			Port:  port,
			Check: check,
		}
		err = sd.Client.Agent().ServiceRegister(reg)
		if err != nil {
			return -1, err
		}
		err = sd.Client.Agent().PassTTL(checkId, "")
		if err != nil {
			return -1, err
		}
	}
	alpha := rand.Float64() * 0.5
	next_lease := math.Max(0.5, alpha*float64(ttl))
	return int(next_lease), nil
}
