package goredis

import (
	"errors"
	"math/rand"
	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"code.byted.org/gopkg/logs"
	"code.byted.org/kv/backoff"
	circuit "code.byted.org/kv/circuitbreaker"
	"code.byted.org/kv/redis-v6/pkg"
	"code.byted.org/kv/redis-v6/pkg/pool"
	"github.com/facebookgo/clock"
)

var ErrServerNotFound = errors.New("redis: not found available server from multi server pool")

/*
 * single server connpool option
 */
type ServOpt struct {
	network     string
	dialTimeout time.Duration
	serv        string           //server ip:port
	breaker     *circuit.Breaker //breaker
}

/* new conn */
func (s *ServOpt) dial() (conn net.Conn, err error) {
	conn, err = net.DialTimeout(s.network, s.serv, s.dialTimeout)
	if err != nil {
		logs.Errorf("DialTimeout failed: server=%s, err=%s", s.serv, err)
		s.breaker.Fail()
	} else {
		s.breaker.Success()
	}
	return
}

/*
 * server pool
 */
type ServPool struct {
	connPool pool.Pooler //pool
	servopt  *ServOpt
	is_dead  bool
}

func NewServPool(server string, opt *Option) *ServPool {
	/* new ServOpt */
	network := opt.Network
	if network == "" {
		network = "tcp"
	}
	buckets := int(opt.windowTime / time.Second)
	if buckets == 0 {
		buckets = 1
	}
	// init & create circuitbreaker
	ck := clock.New()
	bk := backoff.NewExponentialBackOff()
	bk.InitialInterval = opt.initialInterval
	bk.MaxElapsedTime = BACKOFF_MAX_ELAPSE
	bk.Clock = ck
	bk.Reset()
	breaker := circuit.NewBreakerWithOptions(&circuit.Options{
		BackOff:       bk,
		Clock:         ck,
		ShouldTrip:    circuit.RateTripFunc(opt.maxFailureRate, opt.minSample),
		WindowTime:    opt.windowTime,
		WindowBuckets: buckets,
		NeededSuc:     opt.neededSuccesses,
	})
	so := &ServOpt{
		network:     network,
		dialTimeout: opt.DialTimeout,
		serv:        server,
		breaker:     breaker,
	}
	poolopt := &pool.Options{
		Dialer:             so.dial,
		PoolSize:           opt.PoolSize,
		PoolTimeout:        opt.PoolTimeout,
		IdleTimeout:        opt.IdleTimeout,
		LiveTimeout:        opt.LiveTimeout,
		IdleCheckFrequency: 0,
	}
	/* new ServPool */
	serv := &ServPool{
		connPool: pool.NewConnPool(poolopt),
		servopt:  so,
		is_dead:  false,
	}
	return serv
}

/*
 * multi servers pool
 */
type MultiServPool struct {
	servlist []string             //server list
	servmap  map[string]*ServPool //server map
	ch       chan []string        //channel to update server list
	opt      *Option
	cursor   uint32
	sync.RWMutex
}

func shuffleServers(servers []string) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < len(servers); i++ {
		j := r.Intn(len(servers))
		if j != i {
			servers[i], servers[j] = servers[j], servers[i]
		}
	}
}

func addrSplit(addr string) (string, string) {
	i := strings.LastIndex(addr, ":")
	host := strings.TrimFunc(addr[:i], func(r rune) bool {
		return r == '[' || r == ']'
	})
	port := addr[i+1:]
	return host, port
}

func addrJoin(host, port string) string {
	if strings.Contains(host, ":") {
		return "[" + host + "]:" + port
	}
	return host + ":" + port
}

func NewMultiServPool(servers []string, ch chan []string, opt *Option) *MultiServPool {
	/* host to ip */
	var nowservers []string
	servmap := make(map[string]*ServPool)
	for _, serv := range servers {
		host, port := addrSplit(serv)
		iplist, err := net.LookupHost(host)
		if err != nil {
			logs.Warnf("lookup host %s err. err:%s", host, err)
		} else {
			host = iplist[0]
		}
		newserv := addrJoin(host, port)
		nowservers = append(nowservers, newserv)
		servmap[newserv] = NewServPool(newserv, opt)
	}
	shuffleServers(nowservers)
	logs.Infof("host to ip done. servlist:%v", nowservers)

	/* new multi-serv-pool */
	p := &MultiServPool{
		servlist: nowservers,
		ch:       ch,
		opt:      opt,
		cursor:   0,
	}
	p.servmap = servmap

	/* go update server list */
	if opt.autoLoadConf {
		go p.monitorClusterAddrs(opt.autoLoadInterval)
	}

	/* reaper */
	if opt.IdleCheckFrequency > 0 && opt.IdleTimeout > 0 {
		go p.reaper(opt.IdleCheckFrequency)
	}
	return p
}

func (p *MultiServPool) reaper(frequency time.Duration) {
	ticker := time.NewTicker(frequency)
	defer ticker.Stop()

	for range ticker.C {
		for serv, pl := range p.servmap {
			n, err := pl.connPool.(*pool.ConnPool).ReapStaleConns()
			if err != nil {
				logs.Info("ReapStaleConns failed: %s serv: %s", err, serv)
				continue
			}
			s := pl.connPool.Stats()
			if n > 0 {
				logs.Info("reaper: removed %d stale conns (TotalConns=%d FreeConns=%d Requests=%d Hits=%d Timeout=%d serv=%s)",
					n, s.TotalConns, s.FreeConns, s.Requests, s.Hits, s.Timeouts, serv)
			}

		}
	}
}

func (p *MultiServPool) monitorClusterAddrs(interval time.Duration) {
	logs.Info("auto load goroutine is running")
	for {
		select {
		case s := <-p.ch:
			if p.isAddrsChanged(s) {
				p.updateServers(s)
			}
		default:
		}
		time.Sleep(interval / 2)
	}
}

func (p *MultiServPool) isAddrsChanged(newServers []string) bool {
	if len(newServers) != len(p.servlist) {
		return true
	}
	p.RLock()
	oldServers := make([]string, len(p.servlist))
	copy(oldServers, p.servlist)
	p.RUnlock()
	sort.Sort(sort.StringSlice(newServers))
	sort.Sort(sort.StringSlice(oldServers))
	for i := 0; i < len(newServers); i++ {
		if newServers[i] != oldServers[i] {
			return true
		}
	}
	return false
}

/* update server list by servers */
func (p *MultiServPool) updateServers(servers []string) {
	logs.Debugf("update servers: currentServers: %v, newServers: %v", p.servlist, servers)
	oldservnum := len(p.servlist)
	newservnum := len(servers)
	if newservnum < oldservnum/2 {
		logs.Infof("new server list is too little than old. newnum:%d oldnum:%d", newservnum, oldservnum)
		return
	}
	// add new server
	newservpool := make(map[string]*ServPool)
	for _, serv := range servers {
		_, ok := p.servmap[serv]
		if !ok {
			newservpool[serv] = NewServPool(serv, p.opt)
			logs.Infof("add server %s to MultiServPool pool %v", serv, p.servmap[serv])
		} else {
			newservpool[serv] = p.servmap[serv]
			if newservpool[serv].is_dead {
				// not delete when is_dead, just for marking that server reborn, for tracing proxy status
				// DISADISADVANTAGE: occupy memory, bad for random check
				newservpool[serv] = NewServPool(serv, p.opt)
				logs.Infof("mark server %s alive.", serv)
			}
		}
	}
	// mark server dead which not in consul
	for _, oldserv := range p.servlist {
		_, ok := newservpool[oldserv]
		if !ok {
			newservpool[oldserv] = p.servmap[oldserv]
			newservpool[oldserv].is_dead = true
			newservpool[oldserv].connPool.Close()
			logs.Infof("mark server %s dead.", oldserv)
		}
	}
	shuffleServers(servers)
	// update
	p.Lock()
	p.servlist = servers
	p.servmap = newservpool
	p.Unlock()
}

func (p *MultiServPool) getServer() (*ServPool, error) {
	if len(p.servlist) == 0 {
		return nil, ErrServerNotFound
	}
	p.RLock()
	servernum := len(p.servlist)
	p.RUnlock()
	var serverName string
	maxBreakerNum := int(float64(servernum) * p.opt.maxBreakerRatio)
	breakerIndex := 0
	for i := 0; i < servernum; i++ {
		p.RLock()
		index := int(atomic.AddUint32(&p.cursor, 1)) % len(p.servlist)
		serverName = p.servlist[index]
		server, ok := p.servmap[serverName]
		p.RUnlock()

		if !ok {
			logs.Warnf("null serverpool object server: %s", serverName)
			continue
		}
		if server.is_dead {
			logs.Warnf("dead server: %s", serverName)
			continue
		}

		// breaker for connection init and dial connection
		// for some situations init connection can be failed, which may result in starve, client kept connect and disconnect server
		// when the breaker is triggered , client can not dial server for some time

		// to limit the breakers number,so variable i can't be used
		breakerIndex++
		if p.opt.needLimitBreakers && breakerIndex > int(maxBreakerNum) {
			server.connPool.SetRetryDG(true)
			return server, nil
		}

		if !server.servopt.breaker.Ready() {
			server.connPool.SetRetryDG(true)
			//logs.Warnf("Circuit breaker tripped server: %s", server.servopt.serv)
			continue
		}
		if server.connPool.RetryDG() {
			server.connPool.SetRetryDG(false)
		}
		return server, nil
	}
	p.RLock()
	idx := rand.Int() % len(p.servlist)
	serverName = p.servlist[idx]
	serv, ok := p.servmap[serverName]
	p.RUnlock()
	if !ok {
		logs.Warn("all server break and choice server still got break")
		return nil, ErrServerNotFound
	}

	if !serv.connPool.RetryDG() {
		serv.connPool.SetRetryDG(true)
	}
	logs.Noticef("all server break, choice server: %s", serverName)
	return serv, nil
}

/* get conn from MultiServPool */
func (p *MultiServPool) getConnection() (*pool.Conn, bool, *ServPool, error) {
	server, err := p.getServer()
	if err != nil {
		return nil, false, server, err
	}
	cn, isNew, err := server.connPool.Get()
	if err != nil {
		return nil, false, server, err
	}
	return cn, isNew, server, nil
}

/* release conn to MultiServPool or close */
func (p *MultiServPool) releaseConnection(cn *pool.Conn, err error) bool {
	servname := cn.RemoteAddr().String()
	p.RLock()
	serv, ok := p.servmap[servname]
	p.RUnlock()

	if !ok {
		logs.Warnf("resease conn err, no matched server object. servname:%s", servname)
		return false
	}
	if pkg.IsBadConn(err, false) {
		_ = serv.connPool.Remove(cn)
		return false
	}

	_ = serv.connPool.Put(cn)
	return true
}
