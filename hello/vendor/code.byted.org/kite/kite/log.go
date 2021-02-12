package kite

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"code.byted.org/gopkg/logs"
	"code.byted.org/kite/kitc"
	"code.byted.org/gopkg/logs/provider"
)

// NOTE: kite has multiple loggers, using the LogKind to identify
type LogKind int8

const (
	AccessLog LogKind = 1 << iota
	CallLog
	DefaultLog
)

/* NOTE:
 * 可以将 provider 同时增加给多个指定的 kite logger, 用法如下
 * 1. 为单个logger: whichLog = AccessLog
 * 2. 为多个logger: whichLog = AccessLog|CallLog|DefaultLog
 */
func AddLogProvider(whichLog LogKind, provider logs.LogProvider) error {
	var err error
	if whichLog&AccessLog > 0 {
		fmt.Println("KITE: add provider for access log")
		err = addAccessLogProvider(provider)
	}
	if err == nil && whichLog&CallLog > 0 {
		fmt.Println("KITE: add provider for call log")
		err = addCallLogProvider(provider)
	}
	if err == nil && whichLog&DefaultLog > 0 {
		fmt.Println("KITE: add provider for default log")
		err = addDefaultLogProvider(provider)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Add provider error: %s, forget kite.Init() before ?\n", err.Error())
	}
	return err
}

func addAccessLogProvider(provider logs.LogProvider) error {
	if accessLogger != nil {
		return accessLogger.AddProvider(provider)
	}
	return errors.New("accessLogger nil")
}

func addCallLogProvider(provider logs.LogProvider) error {
	if callLogger != nil {
		return callLogger.AddProvider(provider)
	}
	return errors.New("callLogger nil")
}

func addDefaultLogProvider(provider logs.LogProvider) error {
	if defaultLogger != nil {
		return defaultLogger.AddProvider(provider)
	}
	return errors.New("defaultLogger nil")
}

const (
	DATABUS_RPC_PREFIX = "webarch.rpc."
	DATABUS_APP_PREFIX = "webarch.app."
)

var (
	accessLogger  *logs.Logger
	callLogger    *logs.Logger
	defaultLogger *logs.Logger

	databusRPCChannel string
	databusAPPChannel string
)

/* NOTE:
 * 框架 logger 初始化, 包括以下
 * callLogger:   设定 kitc client端调用链日志输出路径(log/rpc/p.s.m.call.log)
 * accessLogger: 设定 kite server端调用链日志输出路径(log/app/p.s.m.access.log)
 * defaultLogger:设定 kite server端默认日志输出策略(提供console/file/databus/agent多种模式,可在yml中配置)
 *
 * 目前 kite 使用 gopkg/logs 库处理日志, 如有自定义logger需求请与我们联系
 */
func initKiteLogger() {
	initAccessLogger()
	initCallLogger()
	initDefaultLogger()
}

func initAccessLogger() {
	filename := filepath.Join(LogDir, "app", ServiceName+".access.log")
	accessLogger = newRPCLogger(filename)
	fmt.Printf("KITE: access log path: %s\n", filename)
}

func initCallLogger() {
	filename := filepath.Join(LogDir, "rpc", ServiceName+".call.log")
	callLogger = newRPCLogger(filename)
	fmt.Printf("KITE: call log path: %s\n", filename)
	kitc.SetCallLog(callLogger)
}

/* NOTE:
 * defaultLog set default logger in logs;
 * user just use logs.Error to do application log;
 */
func initDefaultLogger() {
	defaultLogger = logs.NewLogger(1024)
	defaultLogger.SetLevel(LogLevel)
	defaultLogger.SetCallDepth(3)
	if EnableDyeLog {
		defaultLogger.EnableDynamicLogLevel()
	}

	// NOTE: add providers to defaultLogger
	// NOTE: initFileProvider: we use hourly roated log for the MSP service depend on this
	initFileProvider(defaultLogger, logs.LevelTrace, LogFile, logs.HourDur, MaxLogSize)
	initConsoleProvider(defaultLogger)
	initDatabusProvider(defaultLogger, LogLevel)
	initAgentProvider(defaultLogger, logs.LevelTrace)

	// NOTE: init defaultLogger for kite
	logs.InitLogger(defaultLogger)
}

/* NOTE:
 * newRPCLogger : a logger made for RPC.
 * this logger is no longer a single file logger,
 * we have appended databus & agent provider into it,
 * so the function name has been changed.
 */
func newRPCLogger(file string) *logs.Logger {
	logger := logs.NewLogger(1024)
	logger.SetLevel(logs.LevelTrace)
	logger.SetCallDepth(2)

	fileProvider := logs.NewFileProvider(file, logs.DayDur, 0)
	fileProvider.SetLevel(logs.LevelTrace)
	if err := logger.AddProvider(fileProvider); err != nil {
		fmt.Fprintf(os.Stderr, "Add file provider error: %s\n", err)
		return nil
	}

	if DatabusLog && databusRPCChannel != "" {
		databusProvider := logs.NewDatabusProviderWithChannel(DATABUS_RPC_PREFIX+ServiceName, databusRPCChannel) // 此处为RPC log类型
		databusProvider.SetLevel(logs.LevelTrace)
		if err := logger.AddProvider(databusProvider); err != nil {
			fmt.Fprintf(os.Stderr, "Add databus provider error: %s\n", err)
			return nil
		}
	}

	if AgentLog {
		// Create an agent provider only for RPC log.
		agentProvider := logs.NewRPCLogAgentProvider()
		agentProvider.SetLevel(logs.LevelTrace)
		if err := logger.AddProvider(agentProvider); err != nil {
			fmt.Fprintf(os.Stderr, "Add agent provider error: %s\n", err)
			return nil
		}
	}

	logger.StartLogger()
	return logger
}

// NOTE: add fileProvider into logger
func initFileProvider(logger *logs.Logger, level int, filename string, dur logs.SegDuration, size int64) {
	if FileLog {
		fileProvider := logs.NewFileProvider(filename, dur, size)
		fileProvider.SetLevel(level)
		if err := logger.AddProvider(fileProvider); err != nil {
			fmt.Fprintf(os.Stderr, "Add file provider error: %s\n", err)
		}
	}
}

// NOTE: add consoleProvider into logger
func initConsoleProvider(logger *logs.Logger) {
	if ConsoleLog {
		consoleProvider := logs.NewConsoleProvider()
		consoleProvider.SetLevel(logs.LevelTrace)
		if err := logger.AddProvider(consoleProvider); err != nil {
			fmt.Fprintf(os.Stderr, "Add console provider error: %s\n", err)
		}
	}
}

// NOTE: add databusProvider into logger
func initDatabusProvider(logger *logs.Logger, level int) {
	if DatabusLog && databusAPPChannel != "" {
		databusProvider := logs.NewDatabusProviderWithChannel(DATABUS_APP_PREFIX+ServiceName, databusAPPChannel) // 此处为APP log类型
		databusProvider.SetLevel(level)
		if err := logger.AddProvider(databusProvider); err != nil {
			fmt.Fprintf(os.Stderr, "Add databus provider error: %s\n", err)
		}
	}
}

// NOTE: add agentProvider into logger
func initAgentProvider(logger *logs.Logger, level int) {
	if AgentLog {
		agentProvider := provider.NewLogAgentProvider(ServiceName)
		agentProvider.SetLevel(level)
		if err := logger.AddProvider(agentProvider); err != nil {
			fmt.Fprintf(os.Stderr, "Add agent provider error: %s\n", err)
		}
	}
}
