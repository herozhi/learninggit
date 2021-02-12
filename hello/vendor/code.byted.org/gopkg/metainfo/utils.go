package metainfo

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// HasMetaInfo detects whether the given context contains metainfo.
func HasMetaInfo(ctx context.Context) bool {
	return getKV(ctx) != nil
}

// SetMetaInfoFromMap retrieves metainfo key-value pairs from the given map and sets then into the context.
// Only those keys with prefixes defined in this module would be used.
func SetMetaInfoFromMap(ctx context.Context, m map[string]string) context.Context {
	if ctx == nil {
		return nil
	}
	for k, v := range m {
		if t, nk := determineKeyType(k, v); t != invalidType {
			ctx = addKV(ctx, t, nk, v)
		}
	}
	return ctx
}

// SaveMetaInfoToMap set key-value pairs from ctx to m while filtering out transient-upstream data.
func SaveMetaInfoToMap(ctx context.Context, m map[string]string) {
	if ctx == nil || m == nil {
		return
	}
	ctx = TransferForward(ctx)
	for k, v := range GetAllValues(ctx) {
		m[PrefixTransient+k] = v
	}
	for k, v := range GetAllPersistentValues(ctx) {
		m[PrefixPersistent+k] = v
	}
}

// SetMetaInfoFromHTTPRequest retrieves all metainfo key-value pairs from the headers of the given HTTP request and sets then into the context.
// Only those keys with prefixes defined in this module would be used.
// If a header has multiple values, only the first one will be used.
func SetMetaInfoFromHTTPRequest(ctx context.Context, req *http.Request) context.Context {
	return SetMetaInfoFromHeader(ctx, WrapHttpRequest(req))
}

const (
	lenPTU = len(PrefixTransientUpstream)
	lenPT  = len(PrefixTransient)
	lenPP  = len(PrefixPersistent)
)

// determineKeyType tests whether the given key-value pair is a valid metainfo and returns its info type with a new appropriate key.
func determineKeyType(k, v string) (infoType infoType, newKey string) {
	if len(k) == 0 || len(v) == 0 {
		return invalidType, k
	}

	switch {
	case strings.HasPrefix(k, PrefixTransientUpstream):
		if len(k) > lenPTU {
			return transientUpstreamType, k[lenPTU:]
		}
	case strings.HasPrefix(k, PrefixTransient):
		if len(k) > lenPT {
			return transientType, k[lenPT:]
		}
	case strings.HasPrefix(k, PrefixPersistent):
		if len(k) > lenPP {
			return persistentType, k[lenPP:]
		}
	}
	return invalidType, k
}

type WithHeader interface {
	Header(k string) []string
	ForEachHeader(func(k, v string))
}

type HeaderSetter interface {
	SetHeader(k, v string)
}

const MetaInfoKey = "X-Tt-MetaInfo"

// Set metainfo to HTTP header (HeaderSetter type).
//
// This method will set headers in 2 ways:
//
//   1. the legacy way (the pre-existing way): with metainfo key in the header name;
//      RPC_TRANSIT_${transient-key}: {value}
//      RPC_PERSIST_${persistent-key}: {value}
//
//   2. the new way: with all key/value entries in a single header;
//      X-Tt-MetaInfo: ${meta_json}
//
// [WHY SUPPORT 2 WAYS]
//
// As HTTP header names are case-insensitive (https://tools.ietf.org/html/rfc2616#section-4.2), header in different
// cases are treated as identical. The header name cases may be changed when forwarded by middle HTTP components or
// proxies between the client and server.
//
// In the legacy way, the metainfo variable name are transferred in the HTTP header name. So when the header name case
// is changed, the server (receiver) will not be able to tell the real case of the variable name.
// For example, A metainfo entry with key 'foo' may be received as with key 'Foo'。(Shown as below)
//
//    client: [meta(foo=xxx)] ----(RPC_PERSIST_foo: xxx)----*
//                                                          |
//                                                          V
//                                                  ...middle.proxies...
//                                                          |
//                                                          |
//    server: [meta(Foo=xxx)] <---(Rpc_Persist_Foo: xxx)----*
//
//
// [COMPATIBILITY CONSIDERATION]
//
// As clients may be transferring metainfo in the legacy way already, and servers are parsing those legacy headers,
// we still support the legacy headers. The solution is:
//
// 1. The client will both send headers in the legacy way and the new way.
//
// 2. The server side will first try to parse metainfo in the new way, if such header exists and and is valid, then use
// it. If such header doesn't exist, then try to parse metainfo in the legacy way.
//
// So when either side is using the legacy way (the pre-existing version of gopkg/metainfo), the metainfo will still be
// transferred via the legacy way. And if both sides are using the new way (the new version of gopkg/metainfo), the
// metainfo will be transferred via the new way. So it's safe to upgrade from the legacy way to the new way for both side.
func SetMetaInfoToHeader(ctx context.Context, setter HeaderSetter) {
	if ctx == nil || setter == nil {
		return
	}

	forwarded := TransferForward(ctx)
	transient := GetAllValues(forwarded)
	persist := GetAllPersistentValues(forwarded)

	// the legacy way
	for k, v := range transient {
		setter.SetHeader(HttpTransitPrefix+k, v)
	}
	for k, v := range persist {
		setter.SetHeader(HttpPersistPrefix+k, v)
	}

	// the new way
	meta := make(map[string]string)
	for k, v := range transient {
		meta[PrefixTransient+k] = v
	}
	for k, v := range persist {
		meta[PrefixPersistent+k] = v
	}

	bytes, _ := json.Marshal(meta)
	setter.SetHeader(MetaInfoKey, string(bytes))
}

// Set metainfo to context from HTTP header.
//
// metainfo will be transfered in HTTP header in 2 ways:
//
//   1. the legacy way (the pre-existing way): with metainfo key in the header name;
//      RPC_TRANSIT_${transient-key}: {value}
//      RPC_PERSIST_${persistent-key}: {value}
//
//   2. the new way: with all key/value entries in a single header;
//      X-Tt-MetaInfo: ${meta_json}
//
// [WHY SUPPORT 2 WAYS]
//
// As HTTP header names are case-insensitive (https://tools.ietf.org/html/rfc2616#section-4.2), header in different
// cases are treated as identical. The header name cases may be changed when forwarded by middle HTTP components or
// proxies between the client and server.
//
// In the legacy way, the metainfo variable name are transferred in the HTTP header name. So when the header name case
// is changed, the server (receiver) will not be able to tell the real case of the variable name.
// For example, A metainfo entry with key 'foo' may be received as with key 'Foo'。(Shown as below)
//
//    client: [meta(foo=xxx)] ----(RPC_PERSIST_foo: xxx)----*
//                                                          |
//                                                          V
//                                                  ...middle.proxies...
//                                                          |
//                                                          |
//    server: [meta(Foo=xxx)] <---(Rpc_Persist_Foo: xxx)----*
//
//
// [COMPATIBILITY CONSIDERATION]
//
// As clients may be transferring metainfo in the legacy way already, and servers are parsing those legacy headers,
// we still support the legacy headers. The solution is:
//
// 1. The client will both send headers in the legacy way and the new way.
//
// 2. The server side will first try to parse metainfo in the new way, if such header exists and and is valid, then use
// it. If such header doesn't exist, then try to parse metainfo in the legacy way.
//
// So when either side is using the legacy way (the pre-existing version of gopkg/metainfo), the metainfo will still be
// transferred via the legacy way. And if both sides are using the new way (the new version of gopkg/metainfo), the
// metainfo will be transferred via the new way. So it's safe to upgrade from the legacy way to the new way for both side.
func SetMetaInfoFromHeader(ctx context.Context, header WithHeader) context.Context {
	if ctx == nil || header == nil {
		return ctx
	}

	// first try to parse in the new way
	values := header.Header(MetaInfoKey)
	if len(values) > 0 {
		for _, value := range values {
			m := make(map[string]string)
			e := json.Unmarshal([]byte(value), &m)
			if e == nil {
				for k, v := range m {
					if t, nk := determineKeyType(k, v); t != invalidType {
						ctx = addKV(ctx, t, nk, v)
					}
				}
				return ctx
			}
		}
	}

	// then try to parse in the legacy way
	flags := make(map[string]bool)
	header.ForEachHeader(func(k, v string) {
		// only allow the first value for headers with the same key
		if _, ok := flags[k]; ok {
			return
		}
		flags[k] = true

		if t, nk := determineHttpKeyType(k, v); t != invalidType {
			ctx = addKV(ctx, t, nk, v)
		}
	})

	return ctx
}

// determineKeyType tests whether the given key-value pair is a valid metainfo and returns its info type with a new appropriate key.
func determineHttpKeyType(k, v string) (infoType infoType, newKey string) {
	if len(k) == 0 || len(v) == 0 {
		return invalidType, k
	}

	switch {
	case strings.HasPrefix(k, HttpTransitPrefix):
		if len(k) > LenHttpTransitPrefix {
			return transientType, k[LenHttpTransitPrefix:]
		}
	case strings.HasPrefix(k, HttpPersistPrefix):
		if len(k) > LenHttpPersistPrefix {
			return persistentType, k[LenHttpPersistPrefix:]
		}
	}
	return invalidType, k
}



const (
	// see https://ee.byted.org/madeira/repo/gin/ginex/-/blob/context.go#L48
	HttpPersistPrefix = "Rpc-Persist-"
	HttpTransitPrefix = "Rpc-Transit-"

	LenHttpPersistPrefix = len(HttpPersistPrefix)
	LenHttpTransitPrefix = len(HttpTransitPrefix)
)

func WrapHttpRequest(r *http.Request) WithHeader {
	return &netHttpRequest{r}
}

type netHttpRequest struct {
	*http.Request
}

func (n *netHttpRequest) Header(k string) []string {
	for key, v := range n.Request.Header {
		if strings.EqualFold(key, k) {
			return v
		}
	}
	return nil
}

func (n *netHttpRequest) ForEachHeader(f func(k, v string)) {
	for k, vs := range n.Request.Header {
		for _, v := range vs {
			f(k, v)
		}
	}
}
