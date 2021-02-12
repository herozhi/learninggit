package ginex

import (
	"code.byted.org/gopkg/logs"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"math/rand"

	"code.byted.org/gin/ginex/internal"
	"code.byted.org/gopkg/env"
	"github.com/spf13/viper"
)

const (
	_PRODUCT_MODE        = "Product"
	_DEVELOP_MODE        = "Develop"
	_SERVICE_PORT        = "ServicePort"
	_DEBUG_PORT          = "DebugPort"
	_ENABLE_PPROF        = "EnablePprof"
	_LOG_LEVEL           = "LogLevel"
	_LOG_INTERVAL        = "LogInterval"
	_ENABLE_METRICS      = "EnableMetrics"
	_ENABLE_TRACING      = "EnableTracing"
	_DID_URL_PARAM_KEY   = "DeviceIDParamKey"
	_APPID_URL_PARAM_KEY = "AppIDParamKey"
	_DISABLE_ACCESS_LOG  = "DisableAccessLog"
	_CONSOLE_LOG         = "ConsoleLog"
	_AGENT_LOG           = "AgentLog"
	_FILE_LOG            = "FileLog"
	_MODE                = "Mode"
	_SERVICE_VERSION     = "ServiceVersion"
	_DOMAIN_SOCKET_FMT   = "/opt/tiger/toutiao/var/service/%s.mesh/http.sock"
	_APP_LOG_FILE_COUNT  = "APPLogFileCount"

	_ENV_PSM                   = "PSM"
	_ENV_LOAD_SERVICE_PSM      = "LOAD_SERVICE_PSM"
	_ENV_CONF_DIR              = "GIN_CONF_DIR"
	_ENV_LOG_DIR               = "GIN_LOG_DIR"
	_ENV_SERVICE_PORT          = "RUNTIME_SERVICE_PORT"
	_ENV_DEBUG_PORT            = "RUNTIME_DEBUG_PORT"
	_ENV_HOST_NETWORK          = "IS_HOST_NETWORK"
	_ENV_REQUIRE_HTTP_MESH     = "REQUIRE_HTTP_MESH"
	_ENV_REQUIRE_HTTP_MESH_TCP = "REQUIRE_HTTP_MESH_TCP"
	_ENV_HTTP_PORT             = "HTTP_IPORT"
	_ENV_HTTP_UNIX_PATH        = "HTTP_IUNIX_PATH"
	_ENV_ADDR_PORT_REUSE       = "HTTP_ADDR_PORT_REUSE"
)

var (
	appConfig      AppConfig
	serviceCluster string
	// 存储Flag的临时全局变量，目的是为了让CI平台能将相关的flag注入到私有变量中
	ConfigPSM    string
	ConfigDir    string
	ConfigLogDir string
	ConfigPort   int = -1
)

func init() {
	if strings.EqualFold(os.Getenv(_ENV_REQUIRE_HTTP_MESH), "True") {
		fmt.Fprintf(os.Stdout, "GINEX: http mesh is enabled.\n")
		appConfig.ServiceMeshMode = true
	} else {
		appConfig.ServiceMeshMode = false
	}

	if strings.EqualFold(os.Getenv(_ENV_ADDR_PORT_REUSE), "True") {
		appConfig.bindingAddress = "127.0.0.1"
		appConfig.addrPortReuse = true
	} else {
		appConfig.bindingAddress = "0.0.0.0"
		appConfig.addrPortReuse = false
	}

	if appConfig.ServiceMeshMode {
		//by default, env `REQUIRE_HTTP_MESH_TCP` is absent
		if strings.EqualFold(os.Getenv(_ENV_REQUIRE_HTTP_MESH_TCP), "True") {
			appConfig.networkType = TCP
		} else {
			appConfig.networkType = DomainSocket
		}
	}
}

// 对应框架配置文件配置项
type YamlConfig struct {
	ServicePort           int
	DebugPort             int
	EnablePprof           bool
	LogLevel              string
	LogInterval           string
	EnableMetrics         bool
	DisableAccessLog      bool
	DisableKeepAlive      bool
	ConsoleLog            bool
	AgentLog              bool
	FileLog               bool
	Mode                  string
	ServiceVersion        string
	EnableAntiCrawl       bool
	EnableTracing         bool
	DeviceIDParamKey      string
	AppIDParamKey         string
	HTMLAutoReload        bool
	HTMLReloadIntervalSec int
	EnableDyeLog          bool
	DyeLogLevel           int
	APPLogFileCount       int
}

// 对应框架命令行参数配置项
type FlagConfig struct {
	PSM     string
	ConfDir string
	LogDir  string
	Port    int
}

type NetworkType int32

const (
	TCP          NetworkType = 0
	DomainSocket NetworkType = 1
)

//config resolved at bootstrap stage
type BootstrapConfig struct {
	ServiceMeshMode  bool
	addrPortReuse    bool
	networkType      NetworkType
	bindingAddress   string
	domainSocketPath string
}

type AppConfig struct {
	FlagConfig
	YamlConfig
	BootstrapConfig
}

// PSM return app's PSM
func PSM() string {
	return appConfig.PSM
}

// ConfDir returns the app's config directory. It's a good practice to put all configure files in such directory,
// then you can access config file by filepath.Join(ginex.ConfDir(), "your.conf")
func ConfDir() string {
	return appConfig.ConfDir
}

// LogDir returns app's log root directory
func LogDir() string {
	return appConfig.LogDir
}

// ServiceName return app's service name
func ServiceName() string {
	return appConfig.PSM
}

// ServiceVersion return app's service version
func ServiceVersion() string {
	return appConfig.ServiceVersion
}

// ServicePort returns app's service port
func ServicePort() int {
	return appConfig.ServicePort
}

// DebugPort returns app's debug port
func DebugPort() int {
	return appConfig.DebugPort
}

// EnableWhaleAnticrawl returns true if the config is to enable whale anticrawl
func EnableWhaleAnticrawl() bool {
	return appConfig.EnableAntiCrawl
}

// ServiceMeshMode returns true if mesh mode is on
func ServiceMeshMode() bool {
	return appConfig.ServiceMeshMode
}

//Cluster
func Cluster() string {
	return serviceCluster
}

// config优先级: flag > env > file > default
// env does not work now
func loadConf() {
	// define and parse flags
	parseFlags()

	parseConf()

	parseEnvs()

	parseBootConfs()

	fmt.Fprintf(os.Stdout, "App config: %#v serviceCluster:%s\n", appConfig, serviceCluster)
}

func parseConf() {
	// parse app config
	v := viper.New()
	v.SetEnvPrefix("GIN")

	curConfEnv := GetConfEnv()
	var confFile string
	if len(curConfEnv) == 0 {
		confFile = filepath.Join(ConfDir(), strings.Replace(PSM(), ".", "_", -1)+".yaml")
	} else {
		// Viper 只识别固定格式的后缀
		confFile = filepath.Join(ConfDir(), strings.Replace(PSM(), ".", "_", -1)+"."+curConfEnv+".yaml")
	}

	v.SetConfigFile(confFile)
	if err := v.ReadInConfig(); err != nil {
		msg := fmt.Sprintf("Failed to load app config: %s, %s", confFile, err)
		fmt.Fprintf(os.Stderr, "%s\n", msg)
		panic(msg)
	}

	var mode string
	customMode := GetCustomMode()
	if len(customMode) > 0 {
		mode = customMode
	} else if Product() {
		mode = _PRODUCT_MODE
	} else {
		mode = _DEVELOP_MODE
	}

	vv := v.Sub(mode)
	if vv == nil {
		msg := fmt.Sprintf("Failed to parse config sub module: %s", mode)
		fmt.Fprintf(os.Stderr, "%s\n", msg)
		panic(msg)
	} else {
		setDefault(vv)
	}

	yamlConfig := &appConfig.YamlConfig
	if err := vv.Unmarshal(yamlConfig); err != nil {
		msg := fmt.Sprintf("Failed to unmarshal app config: %s", err)
		fmt.Fprintf(os.Stderr, "%s\n", msg)
		panic(msg)
	}
	parseServicePorts()

}

//parse envs

func parseEnvs() {
	serviceCluster = os.Getenv("SERVICE_CLUSTER")
	if serviceCluster == "" {
		serviceCluster = "default"
	}
}

func parseBootConfs() {

	if unixPath := os.Getenv(_ENV_HTTP_UNIX_PATH); "" != unixPath {
		appConfig.domainSocketPath = unixPath
	} else {
		if env.IsProduct() {
			appConfig.domainSocketPath = fmt.Sprintf(_DOMAIN_SOCKET_FMT, appConfig.PSM)
		} else {
			appConfig.domainSocketPath = fmt.Sprintf("/tmp/ginex_http_mesh_ingress_%d.sock", rand.Int63())
		}
	}
}

// parseServicePorts handles port configs in environment, config file and flag
func parseServicePorts() {
	var err error
	var portEnvVar string
	if appConfig.ServiceMeshMode &&
		appConfig.networkType == TCP &&
		!appConfig.addrPortReuse {
		portEnvVar = _ENV_HTTP_PORT
	} else {
		portEnvVar = _ENV_SERVICE_PORT
	}
	servicePortValue := os.Getenv(portEnvVar)
	debugPortValue := os.Getenv(_ENV_DEBUG_PORT)
	var hostNetWork bool
	if v := os.Getenv(_ENV_HOST_NETWORK); v != "" {
		if hostNetWork, err = strconv.ParseBool(v); err != nil {
			msg := fmt.Sprintf("Failed to convert environment variable: %s, %s", _ENV_HOST_NETWORK, err)
			fmt.Fprintf(os.Stderr, "%s\n", msg)
			panic(msg)
		}
	}

	if hostNetWork {
		// host模式: 只能使用环境变量端口, 否则直接报错
		if port, err := strconv.Atoi(servicePortValue); err != nil {
			msg := fmt.Sprintf("Failed to convert environment variable: %s, %s", _ENV_SERVICE_PORT, err)
			fmt.Fprintf(os.Stderr, "%s\n", msg)
			panic(msg)
		} else {
			appConfig.ServicePort = port
		}

		if debugPortValue == "" {
			appConfig.DebugPort = 0
		} else {
			if port, err := strconv.Atoi(debugPortValue); err != nil {
				msg := fmt.Sprintf("Failed to convert environment variable: %s, %s", _ENV_DEBUG_PORT, err)
				fmt.Fprintf(os.Stderr, "%s\n", msg)
				panic(msg)
			} else {
				appConfig.DebugPort = port
			}
		}
	} else {
		// 非host模式: 如果环境变量指定的端口,使用环境变量的.否则使用配置文件的端口
		if servicePortValue != "" {
			if port, err := strconv.Atoi(servicePortValue); err != nil {
				msg := fmt.Sprintf("Failed to convert environment variable: %s, %s", _ENV_SERVICE_PORT, err)
				fmt.Fprintf(os.Stderr, "%s\n", msg)
				panic(msg)
			} else {
				appConfig.ServicePort = port
			}
		}
		if debugPortValue != "" {
			if port, err := strconv.Atoi(debugPortValue); err != nil {
				msg := fmt.Sprintf("Failed to convert environment variable: %s, %s", _ENV_DEBUG_PORT, err)
				fmt.Fprintf(os.Stderr, "%s\n", msg)
				panic(msg)
			} else {
				appConfig.DebugPort = port
			}
		}
	}

	// flag指定的port优先级最高
	if appConfig.Port != 0 {
		appConfig.ServicePort = appConfig.Port
	}
}

func setDefault(v *viper.Viper) {
	v.SetDefault(_SERVICE_PORT, "6789")
	v.SetDefault(_DEBUG_PORT, "6790")
	v.SetDefault(_ENABLE_PPROF, false)
	v.SetDefault(_LOG_LEVEL, "debug")
	v.SetDefault(_LOG_INTERVAL, "hour")
	v.SetDefault(_ENABLE_METRICS, false)
	v.SetDefault(_ENABLE_TRACING, true)
	v.SetDefault(_DID_URL_PARAM_KEY, "device_id")
	v.SetDefault(_APPID_URL_PARAM_KEY, "aid")
	v.SetDefault(_DISABLE_ACCESS_LOG, false)
	v.SetDefault(_CONSOLE_LOG, true)
	v.SetDefault(_AGENT_LOG, true)
	v.SetDefault(_FILE_LOG, true)
	v.SetDefault(_MODE, "debug")
	v.SetDefault(_SERVICE_VERSION, "0.1.0")
	v.SetDefault(_APP_LOG_FILE_COUNT, 0)
}

func parseFlags() {
	if ConfigPSM == "" {
		flag.StringVar(&ConfigPSM, "psm", "", "psm")
	}
	if ConfigDir == "" {
		flag.StringVar(&ConfigDir, "conf-dir", "", "support config file.")
	}
	if ConfigLogDir == "" {
		flag.StringVar(&ConfigLogDir, "log-dir", "", "log dir")
	}
	if ConfigPort == -1 {
		flag.IntVar(&ConfigPort, "port", 0, "service port")
	}
	if !flag.Parsed() {
		flag.Parse()
	}
	appConfig.PSM = ConfigPSM
	appConfig.ConfDir = ConfigDir
	appConfig.LogDir = ConfigLogDir
	appConfig.Port = ConfigPort

	if appConfig.PSM == "" {
		logs.Warn("PSM not set by CONFIG_FILE. Fallback to next strategy.")
		appConfig.PSM = os.Getenv(_ENV_LOAD_SERVICE_PSM)
	}
	if appConfig.PSM == "" {
		logs.Warn("PSM not set by %s. Fallback to next strategy.", _ENV_LOAD_SERVICE_PSM)
		appConfig.PSM = os.Getenv(_ENV_PSM)
	}
	if appConfig.PSM == "" {
		logs.Warn("PSM not set by %s. Fallback to next strategy.", _ENV_PSM)
		fmt.Fprintf(os.Stderr, "PSM is not specified, use -psm option or %s environment\n", _ENV_PSM)
		usage()
	} else {
		os.Setenv(internal.GINEX_PSM, appConfig.PSM)
	}
	if appConfig.ConfDir == "" {
		appConfig.ConfDir = os.Getenv(_ENV_CONF_DIR)
	}
	if appConfig.ConfDir == "" {
		fmt.Fprintf(os.Stderr, "Conf dir is not specified, use -conf-dir option or %s environment\n", _ENV_CONF_DIR)
		usage()
	}
	if appConfig.LogDir == "" {
		appConfig.LogDir = os.Getenv(_ENV_LOG_DIR)
	}
	if appConfig.LogDir == "" {
		fmt.Fprintf(os.Stderr, "Log dir is not specified, use -log-dir option or %s environment\n", _ENV_LOG_DIR)
		usage()
	}
}

func usage() {
	flag.Usage()
	os.Exit(-1)
}
