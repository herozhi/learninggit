package endpoint

import (
	"context"
)

// EndPoint represent one method for calling from remote.
type EndPoint func(ctx context.Context, req interface{}) (resp interface{}, err error)

// Middleware deal with input EndPoint and output EndPoint
type Middleware func(EndPoint) EndPoint

// Chain connect middlewares into one middleware.
func Chain(outer Middleware, others ...Middleware) Middleware {
	return func(next EndPoint) EndPoint {
		for i := len(others) - 1; i >= 0; i-- {
			next = others[i](next)
		}
		return outer(next)
	}
}

func Build(mws []Middleware) Middleware {
	if len(mws) == 0 {
		return EmptyMiddleware
	}
	return func(next EndPoint) EndPoint {
		return mws[0](Build(mws[1:])(next))
	}
}

func EmptyMiddleware(next EndPoint) EndPoint {
	return next
}

// KiteBase describe the interface of base item in a standard request
type KiteBase interface {
	GetLogID() string
	GetCaller() string
	GetAddr() string
	GetClient() string
	GetEnv() string
	GetCluster() string
}

// KiteBaseExtra describe the interface of base item in a standard request Base Extra
type KiteBaseExtra interface {
	GetExtra() map[string]string
}

// KiteRequest describe the interface of standard request in framework
type KiteRequest interface {
	GetBase() KiteBase
	IsSetBase() bool
	RealRequest() interface{}
}

// KiteResponse describe the interface of standerd response in framework
type KiteResponse interface {
	GetBaseResp() KiteBaseResp
	// RealResponse return a real response instance
	RealResponse() interface{}
}

// KiteBaseResp describe the interface of base response in a standard response
type KiteBaseResp interface {
	GetStatusCode() int32
	GetStatusMessage() string
}

type KitcCallResponse interface {
	GetBaseResp() KiteBaseResp
	RemoteAddr() string
	RealResponse() interface{}
}

type KitcCallRequest interface {
	SetBase(kb KiteBase) error
	RealRequest() interface{}
}

type ThriftBase interface {
	GetLogID() string
	GetCaller() string
	GetAddr() string
	GetClient() string
	GetExtra() map[string]string
}
