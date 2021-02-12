package kiterrno

const (
	// Thrift Application Exception
	UnknwonApllicationException = 0
	UnknownMethod               = 1
	InValidMessageTypeException = 2
	WrongMethodName             = 3
	BadSequenceID               = 4
	MissingResult               = 5
	InternalError               = 6
	ProtocolError               = 7
)

var thriftErrorCodeDesc = map[int]string{
	UnknwonApllicationException: "unknown application exception",
	UnknownMethod:               "unknown method",
	InValidMessageTypeException: "invalid message type",
	WrongMethodName:             "wrong method name",
	BadSequenceID:               "bad sequence ID",
	MissingResult:               "missing result",
	InternalError:               "unknown internal error",
	ProtocolError:               "unknown protocol error",
}
