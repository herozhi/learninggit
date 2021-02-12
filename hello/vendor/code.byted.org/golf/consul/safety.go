package consul

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/user"
	"strings"
)

const processCommandFormat = "/proc/%d/comm"
const amsTagApi = "https://ams.byted.org/api.php?token=6eed2a47fe5e82fa4599b7ca2d6e8838&method=host.tag.get&ip=%s"
const servicePreconditionKey = "service/%s/precondition"

func LocalIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			return ip.String(), nil
		}
	}
	return "", errors.New("are you connected to the network?")
}

func getLocalIp() string {
	ip, err := LocalIP()
	if err != nil {
		return ""
	}
	return ip
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

type AmsTagResponse struct {
	Response map[string][]string
}

func getAmsTag(ip string) []string {
	url := fmt.Sprintf(amsTagApi, ip)
	resp, err := http.Get(url)
	if err != nil {
		return nil
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	response := new(AmsTagResponse)
	json.Unmarshal(body, &response)
	return response.Response["tags"]
}

func loadPrecondition(sd *ServiceDiscovery, service string) map[string][]string {
	precondition := map[string][]string{
		"user":   []string{"tiger"},
		"parent": []string{"supervise", "systemd"},
	}
	pair, _, err := sd.Client.KV().Get(fmt.Sprintf(servicePreconditionKey, service), nil)
	if err == nil && pair != nil {
		json.Unmarshal(pair.Value, &precondition)
	}
	return precondition
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func ensureSafetyInternal(precondition map[string][]string) (safe bool, err error) {
	fmtStr := "safety check failed: type: %s, condition: %s, required conditions: %s"
	for k, v := range precondition {
		if k == "user" && !stringInSlice(getUsername(), v) {
			err = fmt.Errorf(fmtStr, k, getUsername(), v)
			return
		}
		if k == "ip" && !stringInSlice(getLocalIp(), v) {
			err = fmt.Errorf(fmtStr, k, getLocalIp(), v)
			return
		}
		if k == "parent" && !stringInSlice(getParentProcessName(), v) {
			err = fmt.Errorf(fmtStr, k, getParentProcessName(), v)
			return
		}
		if k == "ams_tag" {
			tags := getAmsTag(getLocalIp())
			match := false
			for i := range tags {
				if stringInSlice(tags[i], v) {
					match = true
				}
			}
			if !match {
				err = fmt.Errorf(fmtStr, k, tags, v)
				return
			}
		}
	}
	safe = true
	return
}

func ensureSafety(sd *ServiceDiscovery, service string) (safe bool, err error) {
	precondition := loadPrecondition(sd, service)
	return ensureSafetyInternal(precondition)
}
