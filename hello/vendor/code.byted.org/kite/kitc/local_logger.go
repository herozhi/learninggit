package kitc

import (
	"fmt"
	"os"
)

// localLogger implement TraceLogger interface
type localLogger struct{}

func (l *localLogger) Trace(format string, v ...interface{}) {
	fmt.Fprintf(os.Stdout, "Trace "+format+"\n", v...)
}

func (l *localLogger) Error(format string, v ...interface{}) {
	fmt.Fprintf(os.Stdout, "Error "+format+"\n", v...)
}
