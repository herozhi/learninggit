package goredis

import (
	"math/rand"
	"net"
	"sync"
	"time"

	"sort"

	"code.byted.org/gopkg/logs"
	circuit "code.byted.org/kv/circuitbreaker"
)

type Dialer struct {
	network     string
	dialTimeout time.Duration

	servers  []string
	breakers map[string]*circuit.Breaker
	ch       chan []string
	mutex    sync.Mutex
	index    int

	// circuit breaker options
	maxFailureRate float64
	minSample      int64
	windowTime     time.Duration
	rander         *SafeRander
}

func NewDialer(servers []string, ch chan []string, opt *Option) *Dialer {
	// The network type, either tcp or unix. Default is tcp.
	network := opt.Network
	if network == "" {
		network = "tcp"
	}

	d := &Dialer{
		network:     network,
		dialTimeout: opt.DialTimeout,

		ch:    ch,
		index: 0,

		maxFailureRate: opt.maxFailureRate,
		minSample:      opt.minSample,
		windowTime:     opt.windowTime,
		rander:         NewSafeRander(),
	}
	d.updateServers(servers)

	// TODO: 是不是人为干预连接什么时候关闭?一段时间后关闭还是多少次请求之后关闭?,在goroutine里面搞
	if opt.autoLoadConf {
		go func() {
			logs.Info("auto load goroutine is running")
			for {
				select {
				case s := <-d.ch:
					if len(s) == len(d.servers) {
						curServers := make([]string, len(d.servers))
						copy(curServers, d.servers)
						sort.Sort(sort.StringSlice(s))
						sort.Sort(sort.StringSlice(curServers))
						for i := 0; i < len(s); i++ {
							if s[i] == curServers[i] {
								continue
							}
							d.updateServers(s)
							break
						}
					} else {
						d.updateServers(s)
					}
				default:
				}
				time.Sleep(opt.autoLoadInterval / 2)
			}
		}()
	}
	return d
}

func (d *Dialer) getDialConn() (conn net.Conn, err error) {
	d.mutex.Lock()
	servers := d.servers
	breakers := d.breakers
	index := d.index
	d.index += 1
	d.mutex.Unlock()

	var i int = index
	numServers := int(len(servers))

	for ; i < index+numServers; i++ {
		k := int(i % numServers)
		if !breakers[servers[k]].Ready() {
			logs.Warnf("Circuit breaker tripped server: %v", servers[k])
			d.mutex.Lock()
			d.index += 1
			d.mutex.Unlock()
			continue
		}
		conn, err = net.DialTimeout(d.network, servers[k], d.dialTimeout)
		if err != nil {
			logs.Errorf("DialTimeout failed: server=%s, err=%s", servers[k], err)
			breakers[servers[k]].Fail()
		} else {
			breakers[servers[k]].Success()
		}
		return
	}
	logs.Error("Circuit breaker all servers have tripped. get connect by random server.")
	server := servers[d.rander.Intn(numServers)]
	conn, err = net.DialTimeout(d.network, server, d.dialTimeout)
	if err != nil {
		logs.Errorf("DialTimeout failed: server=%s, err=%s", server, err)
	}
	return
}

func (d *Dialer) createCircuitBreaker() *circuit.Breaker {
	buckets := int(d.windowTime / time.Second)
	if buckets == 0 {
		buckets = 1
	}
	return circuit.NewBreakerWithOptions(&circuit.Options{
		ShouldTrip:    circuit.RateTripFunc(d.maxFailureRate, d.minSample),
		WindowTime:    d.windowTime,
		WindowBuckets: buckets,
	})
}

func (d *Dialer) updateServers(servers []string) {
	logs.Info("update servers: currentServers: %v, newServers: %v", d.servers, servers)
	indexes := rand.Perm(len(servers))
	breakers := make(map[string]*circuit.Breaker)
	newServers := make([]string, 0, len(servers))
	for _, index := range indexes {
		server := servers[index]
		if _, ok := d.breakers[server]; !ok {
			breakers[server] = d.createCircuitBreaker()
		} else {
			breakers[server] = d.breakers[server]
		}
		newServers = append(newServers, server)
	}
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.servers = newServers
	d.breakers = breakers
	d.index = 0
}
