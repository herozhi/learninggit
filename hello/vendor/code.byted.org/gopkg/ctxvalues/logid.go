package ctxvalues

import (
	"context"
)

const CTXKeyLogID = "K_LOGID" // logid key

// SetLogID set logid to context.Context / 在 context.Context 中设置 logid
func SetLogID(ctx context.Context, logid string) context.Context {
	return context.WithValue(ctx, CTXKeyLogID, logid)
}

// LogID get logid from context.Context / 从 context.Context 中获取 logid
func LogID(ctx context.Context) (string, bool) {
	return getStringFromCTX(ctx, CTXKeyLogID)
}
