/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements. See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership. The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package thrift

import (
	"errors"
	"io"
)

var errTransportInterrupted = errors.New("Transport Interrupted")

type Flusher interface {
	Flush() (err error)
}

// Encapsulates the I/O layer
type TTransport interface {
	io.ReadWriteCloser
	Flusher

	// Opens the transport for communication
	Open() error

	// Returns true if the transport is open
	IsOpen() bool
}

type stringWriter interface {
	WriteString(s string) (n int, err error)
}

// This is "enchanced" transport with extra capabilities. You need to use one of these
// to construct protocol.
// Notably, TSocket does not implement this interface, and it is always a mistake to use
// TSocket directly in protocol.
type TRichTransport interface {
	io.ReadWriter
	io.ByteReader
	io.ByteWriter
	stringWriter
	Flusher
}

type HeaderTTransport interface {
	IsNil() bool
	SetClientType(clientType ClientType)
	SetSeqID(seq uint32)
	SeqID() uint32
	Identity() string
	SetIdentity(identity string)
	PeerIdentity() string
	SetHeaders(headers map[string]string)
	SetHeader(key, value string)
	Header(key string) (string, bool)
	Headers() map[string]string
	ClearHeaders()
	SetIntHeader(key uint16, value string)
	SetIntHeaders(headers map[uint16]string)
	IntHeader(key uint16) (string, bool)
	IntHeaders() map[uint16]string
	ClearIntHeaders()
	ReadHeader(key string) (string, bool)
	ReadHeaders() map[string]string
	ReadIntHeader(key uint16) (string, bool)
	ReadIntHeaders() map[uint16]string
	ProtocolID() ProtocolID
	SetProtocolID(protoID ProtocolID) error
	AddTransform(trans TransformID) error
	ResetProtocol() error
	Open() error
	IsOpen() bool
	Close() error
	Read(buf []byte) (int, error)
	ReadByte() (byte, error)
	Write(buf []byte) (int, error)
	WriteByte(c byte) error
	WriteString(s string) (int, error)
	RemainingBytes() uint64
	Flush() error
	ResetFramebuf(r byteReader)
}
