package kitc

type TransCtxKey string

const (
	THeaderInfoIntHeaders = TransCtxKey("theader-int-headers")
	THeaderInfoHeaders    = TransCtxKey("theader-headers")
)

type ProtocolType int

const (
	ProtocolBinary  ProtocolType = 0
	ProtocolCompact ProtocolType = 1
)

// mesh使用的协议版本号
const MeshTHeaderProtocolVersion = "1.0.0"

const (
	MESH_VERSION      uint16 = iota
	TRANSPORT_TYPE
	LOG_ID
	FROM_SERVICE
	FROM_CLUSTER
	FROM_IDC
	TO_SERVICE
	TO_CLUSTER
	TO_IDC
	TO_METHOD
	ENV
	DEST_ADDRESS
	RPC_TIMEOUT
	READ_TIMEOUT
	RING_HASH_KEY
	DDP_TAG
	WITH_MESH_HEADER
	CONN_TIMEOUT
	SPAN_CTX
	SHORT_CONNECTION
	FROM_METHOD
	STRESS_TAG
	MSG_TYPE
	HTTP_CONTENT_TYPE
)

const MESH_OR_TTHEADER_HEADER = "3"

// key of header transport
const (
	HeaderTransRemoteAddr     = "rip"
	HeaderTransToCluster      = "tc"
	HeaderTransToIDC          = "ti"
	HeaderTransPerfTConnStart = "pcs"
	HeaderTransPerfTConnEnd   = "pce"
	HeaderTransPerfTSendStart = "pss"
	HeaderTransPerfTRecvStart = "prs"
	HeaderTransPerfTRecvEnd   = "pre"
	// the connection peer will shutdown later,so it send back the header to tell client to close the connection.
	HeaderConnectionReadyToReset = "crrst"
)
