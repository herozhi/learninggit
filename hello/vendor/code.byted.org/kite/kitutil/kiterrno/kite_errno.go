package kiterrno

const (
	SuccessCode   = 0  // deprecated
	UserErrorCode = -1 // deprecated

	// error codes for circuitbreaker MW
	NotAllowedByServiceCBCode  = 101
	NotAllowedByInstanceCBCode = 102
	RPCTimeoutCode             = 103

	// error codes for degradation MW
	ForbiddenByDegradationCode     = 104
	GetDegradationPercentErrorCode = 105

	// error codes for conn retry MW
	BadConnBalancerCode = 106
	BadConnRetrierCode  = 107
	ConnRetryCode       = 108

	// error codes for rpc retry
	BadRPCRetrierCode = 109 // deprecated
	RPCRetryCode      = 110

	// error codes for common
	NoExpectedFieldCode = 111 // deprecated

	// error codes for pool
	GetConnErrorCode = 112

	// error codes for service discover
	ServiceDiscoverCode = 113

	// error codes for IDC selector
	IDCSelectErrorCode = 114 // deprecated

	// error codes for ACL
	NotAllowedByACLCode = 115

	// error codes for network I/O
	ReadTimeoutCode     = 116 // deprecated, please use RemoteOrNetErrCode
	WriteTimeoutCode    = 117 // deprecated, please use RemoteOrNetErrCode
	ConnResetByPeerCode = 118 // deprecated, please use RemoteOrNetErrCode
	RemoteOrNetErrCode  = 119

	StressBotRejectionCode = 120 // reject stress RPC

	// error code for endpoint qps limit MW
	EndpointQPSLimitRejectCode = 121

	// error code for user error circuitbreaker
	NotAllowedByUserErrCBCode = 122

	// error code for server return nil response and nil error
	ReturnNilRespNilErrCode = 123
)

var kiteErrorCodeDesc = map[int]string{
	NotAllowedByServiceCBCode:      "Not allowed by service circuitbreaker",
	NotAllowedByInstanceCBCode:     "Downstream service's network is bad, not allowed by dialer circuitbreaker",
	RPCTimeoutCode:                 "RPC timeout",
	ForbiddenByDegradationCode:     "Forbidden by degradation",
	GetDegradationPercentErrorCode: "Get degradation percent error",
	BadConnBalancerCode:            "Create Balancer error",
	BadConnRetrierCode:             "Create Conn Retrier error",
	ConnRetryCode:                  "All Conn retries have failed",
	BadRPCRetrierCode:              "Create RPC Retrier error",
	RPCRetryCode:                   "All RPC retries have failed",
	NoExpectedFieldCode:            "No expected field in the context",
	GetConnErrorCode:               "Get connection error",
	ServiceDiscoverCode:            "Service discover error",
	IDCSelectErrorCode:             "Select IDC error",
	NotAllowedByACLCode:            "Not allowed by ACL",
	ReadTimeoutCode:                "Read network timeout",
	WriteTimeoutCode:               "Write network timeout",
	ConnResetByPeerCode:            "Conn reset by peer",
	RemoteOrNetErrCode:             "Remote or network err",
	StressBotRejectionCode:         "Reject stress RPC",
	EndpointQPSLimitRejectCode:     "Reject qps limit",
	NotAllowedByUserErrCBCode:      "Not allowed by user error circuitbreaker",
	ReturnNilRespNilErrCode:        "Server return nil response and nil error",
}

// IsNetErrCode returns if this error is caused by network
func IsNetErrCode(code int) bool {
	return code == GetConnErrorCode
}

// IsKiteErrCode returns if this code is defined and used by kite
func IsKiteErrCode(code int) bool {
	_, ok := kiteErrorCodeDesc[code]
	return ok
}
