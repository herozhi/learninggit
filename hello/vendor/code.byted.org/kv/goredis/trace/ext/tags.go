package ext

import (
	"github.com/opentracing/opentracing-go"
	olog "github.com/opentracing/opentracing-go/log"
)

// These constants define common tag names recommended for better portability across
// tracing systems and languages/platforms.
//
// The tag names are defined as typed strings, so that in addition to the usual use
//
//     span.setTag(TagName, value)
//
// they also support value type validation via this additional syntax:
//
//    TagName.Set(span, value)
//
var (
	//////////////////////////////////////////////////////////////////////
	// common rpc-info related name
	//////////////////////////////////////////////////////////////////////

	RPCLogID     = stringTagName("logID")
	LocalCluster = stringTagName("cluster")
	PeerCluster  = stringTagName("peer.cluster")

	LocalIDC = stringTagName("idc")
	PeerIDC  = stringTagName("peer.idc")

	LocalAddress = stringTagName("address")

	RequestLength  = int32TagName("reqLen")
	ResponseLength = int32TagName("rspLen")

	ReturnCode = int32TagName("retCode")

	//////////////////////////////////////////////////////////////////////
	// event kind enumerate for LogFields
	//////////////////////////////////////////////////////////////////////
	EventKind = eventKindTagName("event")

	// perfT is abbreviation of performanceTimeStart
	// connection event
	EventKindConnectStartEnum = EventKindEnum("perfT.ConnStart")
	EventKindConnectStart     = olog.String(string(EventKind), string(EventKindConnectStartEnum))

	EventKindConnectEndEnum = EventKindEnum("perfT.ConnEnd")
	EventKindConnectEnd     = olog.String(string(EventKind), string(EventKindConnectEndEnum))

	// pkg send-recv event
	EventKindPkgSendStartEnum = EventKindEnum("perfT.SendStart")
	EventKindPkgSendStart     = olog.String(string(EventKind), string(EventKindPkgSendStartEnum))

	EventKindPkgSendEndEnum = EventKindEnum("perfT.SendEnd")
	EventKindPkgSendEnd     = olog.String(string(EventKind), string(EventKindPkgSendEndEnum))

	EventKindPkgRecvStartEnum = EventKindEnum("perfT.RecvStart")
	EventKindPkgRecvStart     = olog.String(string(EventKind), string(EventKindPkgRecvStartEnum))

	EventKindPkgRecvEndEnum = EventKindEnum("perfT.RecvEnd")
	EventKindPkgRecvEnd     = olog.String(string(EventKind), string(EventKindPkgRecvEndEnum))
)

// ---

type stringTagName string

// Set adds a string tag to the `span`
func (tag stringTagName) Set(span opentracing.Span, value string) {
	span.SetTag(string(tag), value)
}

// ---

type int32TagName string

// Set adds a int32 tag to the `span`
func (tag int32TagName) Set(span opentracing.Span, value int32) {
	span.SetTag(string(tag), value)
}

// ---

type uint32TagName string

// Set adds a uint32 tag to the `span`
func (tag uint32TagName) Set(span opentracing.Span, value uint32) {
	span.SetTag(string(tag), value)
}

// ---

type uint16TagName string

// Set adds a uint16 tag to the `span`
func (tag uint16TagName) Set(span opentracing.Span, value uint16) {
	span.SetTag(string(tag), value)
}

// ---

type boolTagName string

// Add adds a bool tag to the `span`
func (tag boolTagName) Set(span opentracing.Span, value bool) {
	span.SetTag(string(tag), value)
}

// ---

type EventKindEnum string

type eventKindTagName string
