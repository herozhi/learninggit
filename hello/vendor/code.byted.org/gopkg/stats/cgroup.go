package stats

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type CGroupCollector struct {
	dirs map[string]string
}

func NewCGroupColletor() (*CGroupCollector, error) {
	selfcgroup, err := ioutil.ReadFile("/proc/self/cgroup")
	if err != nil {
		return nil, err
	}
	c := &CGroupCollector{dirs: make(map[string]string)}
	for _, s := range strings.Split(string(selfcgroup), "\n") {
		// ID:NAME:PATH
		ss := strings.Split(s, ":")
		if len(ss) != 3 {
			continue
		}
		name := ss[1]
		path := filepath.Join("/sys/fs/cgroup/", name, strings.TrimSpace(ss[2]))
		c.dirs[name] = path
	}
	return c, nil
}

type CGroupCPUStat struct {
	NrPeriods     int64
	NrThrottled   int64
	ThrottledTime int64
	CPUTime       time.Duration
}

func (c *CGroupCollector) ReadCPUStat(st *CGroupCPUStat) {
	if c == nil {
		return
	}
	st.NrPeriods = 0
	st.NrThrottled = 0
	st.ThrottledTime = 0
	dir := c.dirs["cpu,cpuacct"]
	if dir == "" {
		return
	}
	data, err := ioutil.ReadFile(filepath.Join(dir, "cpu.stat"))
	if err != nil {
		return
	}
	fmt.Sscanf(string(data),
		"nr_periods %d\nnr_throttled %d\nthrottled_time %d",
		&st.NrPeriods, &st.NrThrottled, &st.ThrottledTime)
	st.CPUTime = c.readCPUTime()
}

func (c *CGroupCollector) readCPUTime() time.Duration {
	dir := c.dirs["cpu,cpuacct"]
	if dir == "" {
		return 0
	}
	data, err := ioutil.ReadFile(filepath.Join(dir, "cpuacct.usage"))
	if err != nil {
		return 0
	}
	i, _ := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	return time.Duration(i)
}
