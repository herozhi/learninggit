package ginex

import (
	"fmt"
	"os"
	"path/filepath"

	"code.byted.org/gopkg/logs"
	"code.byted.org/gopkg/logs/provider"
	"code.byted.org/kite/kitc"
	"code.byted.org/trace/trace-client-go"
	"github.com/opentracing/opentracing-go"
)

const (
	MAX_LOG_SIZE = 1024 * 1024 * 1024
)

var (
	accessLogger *logs.Logger
	appLogger    *logs.Logger
)

func logSegment(logInterval string) logs.SegDuration {
	if logInterval == "hour" {
		return logs.HourDur
	} else if logInterval == "day" {
		return logs.DayDur
	} else {
		return logs.NoDur
	}
}
func logLevelByName(level string) int {
	m := map[string]int{
		"trace":  logs.LevelTrace,
		"debug":  logs.LevelDebug,
		"info":   logs.LevelInfo,
		"notice": logs.LevelNotice,
		"warn":   logs.LevelWarn,
		"error":  logs.LevelError,
		"fatal":  logs.LevelFatal,
	}
	return m[level]
}

func initAccessLogger() {
	if appConfig.DisableAccessLog {
		return
	}
	accessLogger = logs.NewLogger(1024)
	accessLogger.SetLevel(logs.LevelTrace)
	accessLogger.SetCallDepth(3)

	accessLog := filepath.Join(LogDir(), "app", PSM()+".access.log")
	fileProvider := logs.NewFileProvider(accessLog, logSegment(appConfig.LogInterval), 0)
	fileProvider.SetLevel(logs.LevelTrace)
	if err := accessLogger.AddProvider(fileProvider); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add file provider: %s\n", err)
	}

	if appConfig.AgentLog {
		if err := accessLogger.AddProvider(logs.NewRPCLogAgentProvider()); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to add logAgent provider: %s\n", err)
		}
	}

	accessLogger.StartLogger()
}

func initAppLogger() {
	level := logLevelByName(appConfig.LogLevel)

	appLogger = logs.NewLogger(1024)
	appLogger.SetLevel(level)
	appLogger.SetCallDepth(3)
	if appConfig.EnableDyeLog {
		appLogger.EnableDynamicLogLevel()
	}

	if appConfig.FileLog {
		appLog := filepath.Join(LogDir(), "app", PSM()+".log")
		fileProvider := logs.NewFileProvider(appLog, logSegment(appConfig.LogInterval), MAX_LOG_SIZE)
		fileProvider.SetLevel(logs.LevelTrace)

		if appConfig.APPLogFileCount > 0 {
			fileProvider.SetKeepFiles(appConfig.APPLogFileCount)
		}

		if err := appLogger.AddProvider(fileProvider); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to add fileProvider: %s\n", err)
		}
	}
	if appConfig.ConsoleLog {
		consoleProvider := logs.NewConsoleProvider()
		consoleProvider.SetLevel(logs.LevelTrace)
		if err := appLogger.AddProvider(consoleProvider); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to add consoleProvider error: %s\n", err)
		}
	}
	if appConfig.AgentLog {
		agentProvider := provider.NewLogAgentProvider(appConfig.PSM)
		agentProvider.SetLevel(logs.LevelTrace)
		if err := appLogger.AddProvider(agentProvider); err != nil {
			fmt.Fprintf(os.Stderr, "Add agent provider error: %s\n", err)
		}
	}
	appLogger.StartLogger()
}

func initKitcLogger() {
	logDir := filepath.Join(LogDir(), "rpc")
	file := filepath.Join(logDir, PSM()+".call.log")

	kitcLogger := logs.NewLogger(1024)
	kitcLogger.SetLevel(logs.LevelTrace)
	kitcLogger.SetCallDepth(3)

	fileProvider := logs.NewFileProvider(file, logSegment(appConfig.LogInterval), 0)
	fileProvider.SetLevel(logs.LevelTrace)
	if err := kitcLogger.AddProvider(fileProvider); err != nil {
		fmt.Fprintf(os.Stderr, "Add file provider error: %s\n", err)
	}

	if appConfig.AgentLog {
		// Create an agent provider only for RPC log.
		agentProvider := logs.NewRPCLogAgentProvider()
		agentProvider.SetLevel(logs.LevelTrace)
		if err := kitcLogger.AddProvider(agentProvider); err != nil {
			fmt.Fprintf(os.Stderr, "Add agent provider error: %s\n", err)
		}
	}

	kitcLogger.StartLogger()
	kitc.SetCallLog(kitcLogger)
}

func initLog() {
	initAccessLogger()
	initAppLogger()
	initKitcLogger()
	logs.InitLogger(appLogger)
}

func initOpentracing() {
	// init tracing component
	if appConfig.EnableTracing {
		if err := trace.Init(PSM()); err != nil {
			opentracing.SetGlobalTracer(opentracing.NoopTracer{})
		}
		trace.RootSpanEnable(1, trace.CONFIG_FROM_DEFAULT)
	}
}
