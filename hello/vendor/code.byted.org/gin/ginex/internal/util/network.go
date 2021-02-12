package util

import "net"

const (
	UNKNOWN_IP_ADDR = "-"
)

var (
	localIP string
)

// LocalIP returns host's ip
func LocalIP() string {
	return localIP
}

// getLocalIp enumerates local net interfaces to find local ip, it should only be called in init phase
func getLocalIp() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return UNKNOWN_IP_ADDR
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			return ipnet.IP.String()
		}
	}
	return UNKNOWN_IP_ADDR
}

func init() {
	localIP = getLocalIp()
}
