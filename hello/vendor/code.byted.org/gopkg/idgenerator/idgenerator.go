package idgenerator

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"code.byted.org/golf/ssconf"
	"code.byted.org/gopkg/env"
	"code.byted.org/gopkg/logs"
	"code.byted.org/gopkg/metrics"
)

var (
	uriFormat           = "%sgen?ns=%s&cs=%s&count=%d&psm=%s"
	idconf              = "/opt/tiger/ss_conf/ss/id_generator.conf"
	maxIdleConnsPerHost = 5
	maxRetries          = 3
	defaultNeed64Bit    = false
	localPSM            = "unknown"
	version             = "1.0.3"

	httpClient *http.Client

	metricsClient *metrics.MetricsClientV2
)

func init() {
	if kvpairs, err := ssconf.GetTotalConfigure(idconf); err == nil {
		// 生成ID位数, 用于兼容国际化
		if bit, ok := kvpairs["id_length"]; ok && bit == "64" {
			defaultNeed64Bit = true
		}
	}

	httpClient = &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: maxIdleConnsPerHost,
		},
	}
	localPSM = env.PSM()
	metricsClient = metrics.NewDefaultMetricsClientV2("idgenerator", true)
	rand.Seed(time.Now().UnixNano())

	go doUpdateServer()
}

type IdGeneratorClient struct {
	namespace  string
	countspace string
	need64Bit  bool
	timeout    time.Duration
}

//Set64Bits deprecated, reference README of idgenerator
func (ig *IdGeneratorClient) Set64Bits() {
	logs.Error("[idgenerator] Set64Bits is deprecated, reference README of idgenerator")
	ig.need64Bit = true
}

func (ig *IdGeneratorClient) GenMulti(count int) ([]int64, error) {
	if count <= 0 || count > 100 {
		return nil, fmt.Errorf("GenMulti: count is limited to [1, 100]")
	}

	ids := []int64{}
	var uri string
	if ig.need64Bit {
		uri = fmt.Sprintf(uriFormat, "", ig.namespace, ig.countspace, count, localPSM)
	} else {
		uri = fmt.Sprintf(uriFormat, "v0/", ig.namespace, ig.countspace, count, localPSM)
	}

	servers := getServers()
	if len(servers) == 0 {
		metricsClient.EmitCounter("empty_server.error", 1)
		return nil, errors.New("idgenerator empty servers")
	}

	for i := 1; i <= maxRetries; i++ {
		host := servers[rand.Intn(len(servers))]
		url := host + uri

		startTime := time.Now()
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("idgenerator new request err: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), ig.timeout)
		defer cancel()
		req = req.WithContext(ctx)

		if resp, err := httpClient.Do(req); err == nil {
			if body, e := ioutil.ReadAll(resp.Body); e == nil && body != nil && resp.StatusCode == 200 {
				emitMetrics(startTime, true, ig.namespace, ig.countspace, count, ig.need64Bit)
				idstrs := strings.Split(string(body), ",")
				for _, idstr := range idstrs {
					if id, err := strconv.ParseInt(idstr, 10, 64); err == nil {
						ids = append(ids, id)
					}
				}
			} else {
				emitMetrics(startTime, false, ig.namespace, ig.countspace, count, ig.need64Bit)
				logs.Warn("idgenerator read body failed, url: %v, error: %v, body is nil: %v, status_code: %v",
					url, err, body == nil, resp.StatusCode)
			}
			resp.Body.Close()

			if len(ids) > 0 {
				if len(ids) >= count {
					return ids[0:count], nil
				}
				return ids, nil
			}
		} else {
			emitMetrics(startTime, false, ig.namespace, ig.countspace, count, ig.need64Bit)
			logs.Error("idgenerator httpClient get error: %v", err)
		}
	}
	return nil, fmt.Errorf("Internal Error")
}

func (ig *IdGeneratorClient) Gen() (int64, error) {
	ids, err := ig.GenMulti(1)
	if err != nil {
		return 0, err
	}
	return ids[0], err
}

func emitMetrics(st time.Time, success bool, namespace, countspace string, count int, is64Bit bool) {
	var ts []metrics.T
	ts = append(ts, metrics.T{Name: "namespace", Value: namespace})
	ts = append(ts, metrics.T{Name: "countspace", Value: countspace})
	ts = append(ts, metrics.T{Name: "count", Value: strconv.Itoa(count)})
	if is64Bit {
		ts = append(ts, metrics.T{Name: "length", Value: "64"})
	} else {
		ts = append(ts, metrics.T{Name: "length", Value: "52"})
	}
	ts = append(ts, metrics.T{Name: "language", Value: "go"})
	ts = append(ts, metrics.T{Name: "version", Value: version})

	cost := time.Since(st).Nanoseconds() / 1000000

	if success {
		metricsClient.EmitCounter("req.success.throughput", 1, ts...)
		metricsClient.EmitTimer("req.success.latency", cost, ts...)
	} else {
		metricsClient.EmitCounter("req.error.throughput", 1, ts...)
		metricsClient.EmitTimer("req.error.latency", cost, ts...)
	}
}
