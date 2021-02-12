package kite

import (
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	"code.byted.org/gopkg/consul"
	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/logs"
)

const (
	tcePerfTestWhitelistType = "consul"
	processCommandFormat     = "/proc/%d/comm"
	consulUnixSock           = "/opt/tmp/sock/consul.sock"
)

var (
	consulClient    *consul.Consul
	svcDef          consul.ServiceDefinition
	stopRegistering = make(chan bool)

	isPerfTest bool = false
	perfPrefix      = ""
	whitelist       = make(map[string]struct{})
)

func init() {
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

// Register write its name into consul for other services lookup
func Register() error {
	if !env.IsProduct() && os.Getenv("IS_PROD_RUNTIME") == "" {
		// Only register in prod or IS_PROD_RUNTIME is setted
		return nil
	}
	if os.Getenv("IS_LOAD_REGISTERED") == "1" {
		// load script has registed
		return nil
	}

	if ListenType != LISTEN_TYPE_TCP {
		return nil
	}
	if err := ensureSafety(); err != nil {
		return fmt.Errorf("Not safe to register, reason: %s", err)
	}
	consulClient = newConsul()
	registerName := addPerfPrefix(ServiceName)
	registerPort, err := strconv.Atoi(ServicePort)
	if err != nil {
		return fmt.Errorf("parse service port %s", err)
	}
	tags := map[string]string{
		"transport": "thrift.TBufferedTransport",
		"protocol":  "thrift.TBinaryProtocol",
		"version":   ServiceVersion,
		"cluster":   ServiceCluster,
	}
	if ServiceShard != "" {
		tags["shard"] = ServiceShard
	}
	svcDef = consul.ServiceDefinition{
		ID:   fmt.Sprintf("%s-%d", registerName, registerPort),
		Name: registerName,
		Port: registerPort,
		Tags: dumpTags(tags),
	}
	go startRegister()
	return nil
}

func startRegister() {
	attempt := 0
	ttl := 120
	nextLease := 0
	for {
		select {
		case <-stopRegistering:
			logs.Infof("KITE: Stop register %s", svcDef.Name)
			return
		case <-time.After(time.Duration(int(math.Max(float64(nextLease), 1))) * time.Second):
			alpha := rand.Float64() * 0.5
			nextLease = int(math.Max(0.5, alpha*float64(ttl)))
			if err := consulClient.Register(svcDef); err != nil {
				nextLease = int(math.Min(0.2*math.Pow(2, float64(attempt)), float64(ttl)*0.9))
				attempt++
				logs.Errorf("KITE: Register service error: %s, Sleep %d", err, nextLease)
			} else {
				attempt = 0
			}
		}
	}
}

// StopRegister stops register loop and deregisters service
func StopRegister() {
	if consulClient == nil {
		return
	}
	stopRegistering <- true
	if err := consulClient.Deregister(svcDef.Name, svcDef.Port); err != nil {
		logs.Errorf("KITE: failed to deregister service: %s-%d, %s", svcDef.Name, svcDef.Port, err)
	}
}

func addPerfPrefix(name string) string {
	_, ok := whitelist[name]
	if !isPerfTest || ok || strings.HasPrefix(name, perfPrefix) {
		return name
	}

	return perfPrefix + name
}

func newConsul() *consul.Consul {
	return consul.NewConsul(consul.GetAddr())
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

func getUsername() string {
	user, err := user.Current()
	if err != nil {
		return ""
	}
	return user.Username
}

func getProcessCommand(pid int) string {
	fileName := fmt.Sprintf(processCommandFormat, pid)
	contents, err := ioutil.ReadFile(fileName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(contents))
}

func getParentProcessName() string {
	ppid := os.Getppid()
	comm := getProcessCommand(ppid)
	return comm
}

func ensureSafety() error {
	errFmt := "safety check failed: type: %s, condition: %s, required conditions: %s"
	if curUser := getUsername(); curUser != "tiger" {
		return fmt.Errorf(errFmt, "user", curUser, "tiger")
	}
	if parent := getParentProcessName(); !(parent == "supervise" || parent == "systemd") {
		return fmt.Errorf(errFmt, "parent", parent, "[supervise, systemd]")
	}
	return nil
}
