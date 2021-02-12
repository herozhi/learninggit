package ginex

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"

	"code.byted.org/gopkg/consul"
	"code.byted.org/gopkg/logs"
)

const (
	IS_LOAD_REGISTERED = "IS_LOAD_REGISTERED"
)

var (
	consulClient    *consul.Consul
	svcDef          consul.ServiceDefinition
	stopRegistering = make(chan bool)
)

// Register registers service address and useful meta data to consul.
// It's will give up register if:
//   - service has been registered by load.sh
//   - it's not running in product mode
func Register() (err error) {
	if os.Getenv(IS_LOAD_REGISTERED) == "1" {
		logs.Info("Skip self-register: Load has registered")
		return nil
	}
	if !Product() {
		logs.Warn("Skip self-register: not in product environment")
		return nil
	}

	logs.Info("Register service: %s, cluster:%s", PSM(), Cluster())
	consulClient = newConsul()
	registerName := ServiceName()
	registerPort := ServicePort()
	tags := map[string]string{
		"version": ServiceVersion(),
		"cluster": Cluster(),
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
			logs.Infof("GINEX: Stop register %s", svcDef.Name)
			return
		case <-time.After(time.Duration(int(math.Max(float64(nextLease), 1))) * time.Second):
			alpha := rand.Float64() * 0.5
			nextLease = int(math.Max(0.5, alpha*float64(ttl)))
			if err := consulClient.Register(svcDef); err != nil {
				nextLease = int(math.Min(0.2*math.Pow(2, float64(attempt)), float64(ttl)*0.9))
				attempt++
				logs.Errorf("GINEX: Register service error: %s, Sleep %d", err, nextLease)
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
		logs.Errorf("GINEX: failed to deregister service: %s-%d, %s", svcDef.Name, svcDef.Port, err)
	}
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
