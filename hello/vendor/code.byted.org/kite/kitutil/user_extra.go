package kitutil

import "context"

const (
	kvCtxKey = "K_KV"
)

/*
	在实现UserExtra功能时, 为了满足原生context继承的语义, 引入此结构体
*/
type ctxKV struct {
	k   string
	v   string
	pre *ctxKV
}

func ctxAddKV(ctx context.Context, k, v string) context.Context {
	if ctx == nil {
		return nil
	}

	return context.WithValue(ctx, kvCtxKey, &ctxKV{
		k:   k,
		v:   v,
		pre: ctxGetKV(ctx),
	})

}

func ctxGetKV(ctx context.Context) *ctxKV {
	if ctx == nil {
		return nil
	}
	i := ctx.Value(kvCtxKey)
	if i == nil {
		return nil
	}
	if kv, ok := i.(*ctxKV); ok {
		return kv
	}
	return nil
}

func ctxGetV(ctx context.Context, k string) (string, bool) {
	kv := ctxGetKV(ctx)
	if kv == nil {
		return "", false
	}

	for kv != nil {
		if kv.k == k {
			return kv.v, true
		}
		kv = kv.pre
	}

	return "", false
}

func ctxGetAll(ctx context.Context) map[string]string {
	if ctx == nil {
		return nil
	}
	kv := ctxGetKV(ctx)
	if kv == nil {
		return nil
	}

	results := make(map[string]string, 4)
	for kv != nil {
		if _, exist := results[kv.k]; !exist {
			results[kv.k] = kv.v
		}
		kv = kv.pre
	}

	return results
}
