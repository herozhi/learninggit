package utils

import (
	"strconv"
	"time"

	"github.com/opentracing/opentracing-go"
	olog "github.com/opentracing/opentracing-go/log"
)

// AppendLogRecord append log to finish options by microsecond timestamp
func AppendLogRecordByTimeString(finishOpts *opentracing.FinishOptions, field *olog.Field, timeStamp string) {
	us, err := strconv.ParseInt(timeStamp, 10, 64)
	if err != nil {
		return
	}
	AppendLogRecord(finishOpts, field, time.Unix(0, us*1000))
}

func AppendLogRecord(finishOpts *opentracing.FinishOptions, field *olog.Field, timeStamp time.Time) {
	if finishOpts == nil || field == nil {
		return
	}
	finishOpts.LogRecords = append(finishOpts.LogRecords, opentracing.LogRecord{
		Timestamp: timeStamp,
		Fields:    []olog.Field{*field},
	})
}