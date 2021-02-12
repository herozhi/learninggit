package kitutil

import (
	"context"

	"code.byted.org/gopkg/metainfo"
)

// The prefix listed below may be used to tag the types of values when there is no context to carry them.
const (
	PrefixPersist         = metainfo.PrefixPersistent
	PrefixTransit         = metainfo.PrefixTransient
	PrefixTransitUpstream = metainfo.PrefixTransientUpstream
)

// Using empty string as key or value is not support.

// GetValue retrieves the value set into the context by given key.
func GetValue(ctx context.Context, k string) (string, bool) {
	return metainfo.GetValue(ctx, k)
}

// GetAllValues retrieves all transient values
func GetAllValues(ctx context.Context) map[string]string {
	return metainfo.GetAllValues(ctx)
}

// WithValue sets the value into the context by given key.
// This value will be propagated to the next service/endpoint through a RPC call.
//
// Notice that it will not propagate any further beyond the next service/endpoint,
// Use WithPersistValue if you want to pass a key/value pair all the way.
func WithValue(ctx context.Context, k string, v string) context.Context {
	return metainfo.WithValue(ctx, k, v)
}

// DelValue deletes a key/value from current context.
// Since empty string value is not valid, we could just set the value to be empty.
func DelValue(ctx context.Context, k string) context.Context {
	return metainfo.DelValue(ctx, k)
}

// GetPersistValue retrieves the persistent value set into the context by given key.
func GetPersistValue(ctx context.Context, k string) (string, bool) {
	return metainfo.GetPersistentValue(ctx, k)
}

// GetAllPersistValues retrieves all persistent value
func GetAllPersistValues(ctx context.Context) map[string]string {
	return metainfo.GetAllPersistentValues(ctx)
}

// WithPersistValue sets the value info the context by given key.
// This value will be propagated to the services along the RPC call chain.
func WithPersistValue(ctx context.Context, k string, v string) context.Context {
	return metainfo.WithPersistentValue(ctx, k, v)
}

// DelPersistValue deletes a persistent key/value from current context.
// Since empty string value is not valid, we could just set the value to be empty.
func DelPersistValue(ctx context.Context, k string) context.Context {
	return metainfo.DelPersistentValue(ctx, k)
}
