package gorm

import (
	"context"
	"database/sql/driver"
	"fmt"
	"reflect"
	"regexp"
	"time"
	"unicode"

	"code.byted.org/gopkg/logs"
)

var (
	defaultLogger = Logger{}
	sqlRegexp     = regexp.MustCompile(`(\$\d+)|\?`)
)

type logger interface {
	Print(ctx context.Context, v ...interface{})
}

// Logger default logger
type Logger struct{
	extLogger *logs.Logger
}

// Print format & print log
func (logger Logger) Print(ctx context.Context, values ...interface{}) {
	if len(values) > 1 {
		var message string
		level := values[0]

		if level == "sql" {
			// sql
			var sql string
			var formattedValues []string
			// duration
			cost := float64(values[2].(time.Duration).Nanoseconds()/1e4) / 100.0

			for _, value := range values[4].([]interface{}) {
				indirectValue := reflect.Indirect(reflect.ValueOf(value))
				if indirectValue.IsValid() {
					value = indirectValue.Interface()
					if t, ok := value.(time.Time); ok {
						formattedValues = append(formattedValues, fmt.Sprintf("'%v'", t.Format(time.RFC3339)))
					} else if b, ok := value.([]byte); ok {
						if str := string(b); isPrintable(str) {
							formattedValues = append(formattedValues, fmt.Sprintf("'%v'", str))
						} else {
							formattedValues = append(formattedValues, "'<binary>'")
						}
					} else if r, ok := value.(driver.Valuer); ok {
						if value, err := r.Value(); err == nil && value != nil {
							formattedValues = append(formattedValues, fmt.Sprintf("'%v'", value))
						} else {
							formattedValues = append(formattedValues, "NULL")
						}
					} else {
						formattedValues = append(formattedValues, fmt.Sprintf("'%v'", value))
					}
				} else {
					formattedValues = append(formattedValues, fmt.Sprintf("'%v'", value))
				}
			}

			var formattedValuesLength = len(formattedValues)
			for index, value := range sqlRegexp.Split(values[3].(string), -1) {
				sql += value
				if index < formattedValuesLength {
					sql += formattedValues[index]
				}
			}
			message = fmt.Sprintf("SQL:%s Cost:%.2fms", sql, cost)
		} else {
			message = fmt.Sprint(values[2:]...)
		}

		if logger.extLogger == nil {
			logs.CtxInfo(ctx, "GORM LOG %s", message)
		} else {
			logger.extLogger.CtxInfo(ctx, "GORM LOG %s", message)
		}

	}
}

func isPrintable(s string) bool {
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}
