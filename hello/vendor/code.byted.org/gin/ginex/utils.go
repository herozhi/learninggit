package ginex

import (
	"net"
	"strings"

	"code.byted.org/gin/ginex/internal"
	"github.com/gin-gonic/gin"
)

func GetClientIP(ctx *gin.Context) string {
	// According to 王宁's email:《Fwd: 动态加速可能需要业务逻辑做些调整》
	// Ali-CDN-Real-IP has the highest priority
	clientIP := ctx.GetHeader("Ali-CDN-Real-IP")
	if isValidPublicIP(clientIP) {
		return clientIP
	}

	ips := ctx.GetHeader("X-Forwarded-For")
	if len(ips) != 0 {
		ipArray := strings.Split(ips, ",")

		for _, rawClientIP := range ipArray {
			clientIP := strings.TrimSpace(rawClientIP)
			if isValidPublicIP(clientIP) {
				return clientIP
			}
		}
	}

	clientIP = ctx.GetHeader("X-Real-IP")
	if isValidPublicIP(clientIP) {
		return clientIP
	}

	// 对于小运营商，上述逻辑可能获取内网IP。此种场景可通过x-alicdn-da-via获取公网IP
	ips = ctx.GetHeader("x-alicdn-da-via")
	if len(ips) != 0 {
		ipArray := strings.Split(ips, ",")

		for _, rawClientIP := range ipArray {
			clientIP := strings.TrimSpace(rawClientIP)
			if isValidPublicIP(clientIP) {
				return clientIP
			}
		}
	}

	// try to call gin.Context.ClientIP after all
	return ctx.ClientIP()
}

func isValidPublicIP(rawIP string) bool {
	ip := net.ParseIP(rawIP)

	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		switch true {
		case ip4[0] == 10:
			return false
		case ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31:
			return false
		case ip4[0] == 192 && ip4[1] == 168:
			return false
		default:
			return true
		}
	}
	return true
}

// warning: do not use this Func unless you know what you are doing.
func SetCustomHandlerName(h gin.HandlerFunc, n string) {
	internal.SetHandlerName(h, n)
}

// for path pattern mapping
func GetRealHandlerName(handlerFunc gin.HandlerFunc, fullName string) string {
	method := internal.GetHandlerName(handlerFunc)
	if method == "" {
		method = fullName
		pos := strings.LastIndexByte(method, '.')
		if pos != -1 {
			method = fullName[pos+1:]
		}
	}
	return method
}
