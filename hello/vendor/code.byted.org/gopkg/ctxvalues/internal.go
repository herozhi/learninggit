package ctxvalues

import (
	"context"
)

func getStringFromCTX(ctx context.Context, key string) (string, bool) {
	if ctx == nil {
		return "", false
	}

	v := ctx.Value(key)
	if v == nil {
		return "", false
	}

	switch v := v.(type) {
	case string:
		return v, true
	case *string:
		return *v, true
	}
	return "", false
}
