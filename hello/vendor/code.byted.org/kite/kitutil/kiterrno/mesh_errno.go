package kiterrno

const (
	// Mesh proxy error codes
	ProxyUnknownErr             = 1101
	ProxyInternalErr            = 1102
	ProxyConnTimeout            = 1111
	ProxyReadTimeout            = 1112
	ProxyWriteTimeout           = 1113
	CallConnTimeout             = 1201
	CallReadTimeout             = 1202
	CallWriteTimeout            = 1203
	CallTotalTimeout            = 1204
	BadProtocolErr              = 1301
	SerializeErr                = 1302
	DeserializeErr              = 1303
	ProxyTransportErr           = 1401
	CallerTransportErr          = 1402
	CalleeTransportErr          = 1403
	ServiceDiscoveryInternalErr = 1501
	ServiceDiscoveryEmptyErr    = 1502
	NotAllowedByACL             = 1601
	DegradeDrop                 = 1602
	OverConnectionLimit         = 1603
	OverQPSLimit                = 1604
	CircuitbreakerOpen          = 1605
	OverloadProtectionDeny      = 1606
	UpstreamClose               = 1701
	ACLTokenParseFailed         = 1801
	ACLTokenVerifyFailed        = 1802
)

var meshErrorCodeDesc = map[int]string{
	ProxyUnknownErr:             "Proxy unknown error",
	ProxyInternalErr:            "Proxy internal error",
	ProxyConnTimeout:            "Proxy connection timeout",
	ProxyReadTimeout:            "Proxy read timeout",
	ProxyWriteTimeout:           "Proxy write timeout",
	CallConnTimeout:             "Call connection timeout",
	CallReadTimeout:             "Call read timeout",
	CallWriteTimeout:            "Call write timeout",
	CallTotalTimeout:            "Call rpc timeout",
	BadProtocolErr:              "Bad protocol",
	SerializeErr:                "Serialization error",
	DeserializeErr:              "Deserialization error",
	ProxyTransportErr:           "Proxy transport error",
	CallerTransportErr:          "Caller transport error",
	CalleeTransportErr:          "Callee transport error",
	ServiceDiscoveryInternalErr: "Service discovery internal error",
	ServiceDiscoveryEmptyErr:    "Service discovery empty",
	NotAllowedByACL:             "Not allowed by ACL",
	DegradeDrop:                 "Dropped by degration",
	OverConnectionLimit:         "Connection over limit",
	OverQPSLimit:                "QPS over limit",
	CircuitbreakerOpen:          "Forbidden by circuitbreak",
	OverloadProtectionDeny:      "OverloadProtectionDeny",
	UpstreamClose:               "Upstream closed",
	ACLTokenParseFailed:         "ACL token parse error",
	ACLTokenVerifyFailed:        "ACL token verify error",
}

// IsMeshErrorCode return true if the given code is a possible mesh error code
func IsMeshErrorCode(code int) bool {
	return 1000 < code && code < 10000
}
