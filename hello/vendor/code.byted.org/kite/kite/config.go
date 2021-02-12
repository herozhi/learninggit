package kite

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"code.byted.org/gopkg/env"
)

const (
	DefaultMaxConns              int64 = 10000
	DefaultLimitQps              int64 = 50000
	DefaultQPSFillMS             int   = 100
	DefaultTransportBufferedSize int   = 4096

	_ENV_CONFIG_FILE  = "KITE_CONFIG_FILE"
	_ENV_LOG_DIR      = "KITE_LOG_DIR"
	_ENV_SERVICE_NAME = "KITE_SERVICE_NAME"
	_ENV_SERVICE_PORT = "KITE_SERVICE_PORT"
	_ENV_DEBUG_PORT   = "KITE_DEBUG_PORT"

	_TCE_SERVICE_PORT = "RUNTIME_SERVICE_PORT"

	LISTEN_TYPE_TCP  = "tcp"
	LISTEN_TYPE_UNIX = "unix"
)

var (
	ServiceConfig ConfigInterface
	// 服务名称 psm
	ServiceName string
	// 当前服务版本号，由commit ID的前缀和编译时间组成, 编译的时候打入
	ServiceVersion string = "DefaultVersion"
	// RPC配置文件目录，渐渐会废弃掉
	RpcConfDir string
	// 启动时间
	StartTime time.Time
	// 服务当前所在集群
	ServiceCluster string
	// 服务实例分片
	ServiceShard string

	// tcp or unix domain socket
	ListenType string
	// 服务IP地址
	ServiceAddr string
	// 服务端口
	ServicePort string

	ReadWriteTimeout time.Duration

	EnableMonitor     bool
	MonitorHostPort   string
	EnableDebugServer bool
	EnableMetrics     bool
	EnableTracing     bool
	DebugServerPort   string
	ServicePath       string
	ConfigFile        string
	ConfigDir         string
	ConfigEnv         string
	LogDir            string
	LogLevel          int

	// dye log
	EnableDyeLog bool
	DyeLogLevel  int

	LogFile     string
	MaxLogSize  int64
	LogInterval string

	DisableAccessLog bool

	// log provider
	ConsoleLog   bool
	ScribeLog    bool
	FileLog      bool
	DatabusLog   bool
	AgentLog     bool
	ExitWaitTime time.Duration

	LocalIp string // machine IP

	limitQPS         int64
	limitQPSInterval time.Duration
	limitMaxConns    int64

	ServiceMeshMode        bool
	ServiceMeshIngressAddr string

	enableGLS    bool
	GetRealIP    bool
	NoPushNotice bool
	runEnvKey    string

	// This option forces databus channel for logs to be closed
	// despite of the `DatabusLog` options.
	DisableDatabus bool = true
)

func Usage() {
	usage := `
	-conf  config file
	-log   log dir
	-svc   svc name
	-port  listen port
	-rpc   rpc conf dir
	`
	fmt.Fprintln(os.Stderr, os.Args[0], usage)
	os.Exit(-1)
}

func initFromArgs() {
	if ConfigFile == "" {
		flag.StringVar(&ConfigFile, "conf", "", "support config file.")
	}
	if LogDir == "" {
		flag.StringVar(&LogDir, "log", "", "support log dir.")
	}
	if ServiceName == "" {
		flag.StringVar(&ServiceName, "svc", "", "support svc name.")
	}
	if ServicePort == "" {
		flag.StringVar(&ServicePort, "port", "", "support service port")
	}
	if RpcConfDir == "" {
		flag.StringVar(&RpcConfDir, "rpc", "", "support rpc conf dir")
	}
	flag.Parse()
	ConfigDir = path.Dir(ConfigFile)
}

func initFromEnvs() {
	// 用环境变量中的值覆盖命令行参数传入
	if v := env.PSM(); v != env.PSMUnknown {
		ServiceName = v
	}
	if v := os.Getenv(_ENV_SERVICE_NAME); v != "" {
		ServiceName = v
	}
	if v := os.Getenv(_ENV_SERVICE_PORT); v != "" {
		ServicePort = v
	}
	if v := os.Getenv(_TCE_SERVICE_PORT); v != "" {
		ServicePort = v
	}
	ServiceCluster = os.Getenv("SERVICE_CLUSTER")
	if ServiceCluster == "" {
		ServiceCluster = "default"
	}

	if v := os.Getenv("SHARD_INDEX"); v != "" {
		ServiceShard = v
	}

	ConfigEnv = os.Getenv("CONF_ENV")

	var err error
	ConfigFile, err = filepath.Abs(ConfigFile)
	if err != nil {
		panic(fmt.Errorf("Get abs config file error: %s", err))
	}
	if ConfigEnv != "" {
		ConfigFile = ConfigFile + "." + ConfigEnv
	}

	LogDir, err = filepath.Abs(LogDir)
	if err != nil {
		panic(fmt.Errorf("Get abs log dir error: %s", err))
	}
	LogFile = filepath.Join(LogDir, "app", ServiceName+".log")
	LocalIp = env.HostIP()

	if !ServiceMeshMode && os.Getenv("SERVICE_MESH_INGRESS_ADDR") != "" {
		ServiceMeshMode = true
		ServiceMeshIngressAddr = os.Getenv("SERVICE_MESH_INGRESS_ADDR")
	}
}

func initDatabusChannel() {
	switch env.IDC() {
	case env.DC_LF, env.DC_HY, env.DC_HL:
		databusAPPChannel = "__LOG__"
		databusRPCChannel = "web_rpc_log"
		// support test env
		testPrefix := os.Getenv("TESTING_PREFIX")
		if testPrefix != "" {
			databusAPPChannel = testPrefix + "_" + "normal_log"
			databusRPCChannel = testPrefix + "_" + databusRPCChannel
		}
	default:
		databusAPPChannel = env.IDC() + "_web_normal_log"
		databusRPCChannel = env.IDC() + "_web_rpc_log"
	}
}

func initFromConfFile() {
	var err error
	cfg, err := NewYamlFromFile(ConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "can not parse config file %s\n", ConfigFile)
		os.Exit(-1)
	}

	if runEnvKey == "" {
		switch {
		case env.IsProduct():
			runEnvKey = "Product"
		case env.IsTesting():
			runEnvKey = "Testing"
		default:
			runEnvKey = "Develop"
		}
	}

	item := GetConfigItem(cfg, runEnvKey)
	if item != nil {
		ServiceConfig = item
	} else {
		fmt.Fprintf(os.Stderr, "no %s-env config, Develop-env config will be used\n", runEnvKey)
		ServiceConfig = GetConfigItem(cfg, "Develop")
	}

	if ServicePort == "" {
		ServicePort = ServiceConfig.DefaultString("ServicePort", ServicePort)
	}

	ListenType = ServiceConfig.DefaultString("ListenType", LISTEN_TYPE_TCP)
	ServiceAddr = ServiceConfig.DefaultString("ServiceAddr", "")
	EnableMetrics = ServiceConfig.DefaultBool("EnableMetrics", false)
	EnableTracing = ServiceConfig.DefaultBool("EnableTracing", true)
	EnableDebugServer = ServiceConfig.DefaultBool("EnableDebugServer", true)

	ConsoleLog = ServiceConfig.DefaultBool("ConsoleLog", true)
	ScribeLog = ServiceConfig.DefaultBool("ScribeLog", false)
	FileLog = ServiceConfig.DefaultBool("FileLog", false)
	DatabusLog = ServiceConfig.DefaultBool("DatabusLog", false) // 默认Databus关闭
	AgentLog = ServiceConfig.DefaultBool("AgentLog", true)      // 默认向logagent打
	DisableDatabus = ServiceConfig.DefaultBool("DisableDatabus", true)
	if DisableDatabus {
		DatabusLog = false
	}
	DisableAccessLog = ServiceConfig.DefaultBool("DisableAccessLog", false)
	ExitWaitTimeStr := ServiceConfig.DefaultString("ExitWaitTime", "5s")
	if dur, err := time.ParseDuration(ExitWaitTimeStr); err == nil {
		ExitWaitTime = dur
	} else {
		fmt.Fprintf(os.Stderr, "invalid exit wait time %s, default value 5s will be used\n", ExitWaitTimeStr)
		ExitWaitTime = time.Second * 5
	}

	DebugServerPort = ServiceConfig.DefaultString("DebugServerPort", "1"+ServicePort) // 默认pprof地址
	if debugPort := env.TCEDebugPort(); debugPort != "" {
		DebugServerPort = debugPort
	}
	if debugPort := os.Getenv(_ENV_DEBUG_PORT); len(debugPort) != 0 {
		DebugServerPort = debugPort
	}

	LogLevel = ServiceConfig.DefaultInt("LogLevel", 0)                    // 默认使用Trace等级
	MaxLogSize = ServiceConfig.DefaultInt64("MaxLogSize", 1024*1024*1024) // 1G
	LogInterval = ServiceConfig.DefaultString("LogInterval", "day")       // 默认按天切分
	EnableDyeLog = ServiceConfig.DefaultBool("EnableDyeLog", false)
	DyeLogLevel = ServiceConfig.DefaultInt("DyeLogLevel", 1) // 默认使用Debug等级

	defaultReadWriteTimeout := "3s"
	if ServiceMeshMode {
		defaultReadWriteTimeout = "10s"
	}
	duration := ServiceConfig.DefaultString("ReadWriteTimeout", defaultReadWriteTimeout)
	ReadWriteTimeout, err = time.ParseDuration(duration)
	if err != nil {
		ReadWriteTimeout, _ = time.ParseDuration(defaultReadWriteTimeout)
	}

	limitQPS = ServiceConfig.DefaultInt64("LimitQPS", DefaultLimitQps)
	limitQPSFillMS := ServiceConfig.DefaultInt("limitQPSFillMS", DefaultQPSFillMS)
	limitQPSInterval = time.Duration(limitQPSFillMS) * time.Millisecond
	limitMaxConns = ServiceConfig.DefaultInt64("LimitConnections", DefaultMaxConns)

	if !ServiceMeshMode {
		ServiceMeshIngressAddr = ServiceConfig.DefaultString("ServiceMeshIngressAddr", "")
		if ServiceMeshIngressAddr != "" {
			ServiceMeshMode = true
		}
	}
	GetRealIP = ServiceConfig.DefaultBool("GetRealIP", false)
	NoPushNotice = ServiceConfig.DefaultBool("NoPushNotice", false)
}
