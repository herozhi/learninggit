package trace

import (
	"code.byted.org/gopkg/logs"
)

var BytedLogger = &bytedLogger{}

type bytedLogger struct{}

// change error msg into byted warn message
func (l *bytedLogger) Error(msg string) {
	logs.Warn("%s", msg)
}

// chang info msg into byted debug message
func (l *bytedLogger) Infof(format string, args ...interface{}) {
	logs.Debug(format, args...)
}
