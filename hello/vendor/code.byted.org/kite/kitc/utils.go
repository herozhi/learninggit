package kitc

import (
	"errors"
	"reflect"
	"runtime"
	"strings"
)

func joinErrs(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}

	s := make([]string, len(errs))
	for i, e := range errs {
		s[i] = e.Error()
	}
	return errors.New(strings.Join(s, ","))
}

func ifelse(ok bool, onTrue, onFalse string) string {
	if ok {
		return onTrue
	} else {
		return onFalse
	}
}

func getFuncName(fv reflect.Value) string {
	if f := runtime.FuncForPC(fv.Pointer()); f != nil {
		return f.Name()
	}
	return ""
}

// see comments in kitc.client.go
type _lbKey struct{}

var lbKey _lbKey
