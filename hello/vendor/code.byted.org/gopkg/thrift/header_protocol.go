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
	"fmt"
	"strconv"
)

const (
	INT_HEADER_MSG_TYPE uint16 = 22
)

type HeaderProtocol struct {
	TProtocol
	origTransport TTransport
	trans         HeaderTTransport

	protoID ProtocolID
}

type HeaderProtocolFactory struct{
	protoID ProtocolID
}

func NewHeaderProtocolFactory(protoID ProtocolID) *HeaderProtocolFactory {
	return &HeaderProtocolFactory{protoID: protoID}
}

func (p *HeaderProtocolFactory) GetProtocol(trans TTransport) TProtocol {
	tp := &HeaderProtocol{
		origTransport: trans,
		protoID:       p.protoID,
	}
	if et, ok := trans.(HeaderTTransport); ok {
		tp.trans = et
	} else {
		tp.trans = NewHeaderTransport(trans)
	}
	if tp.trans.IsNil() {
		return tp
	}

	tp.trans.SetProtocolID(p.protoID) // reset transport protoID
	if tp.TProtocol != nil {
		return tp
	}

	switch p.protoID {
	case ProtocolIDBinary:
		// These defaults match cpp implementation
		tp.TProtocol = NewTBinaryProtocol(tp.trans, false, true)
	case ProtocolIDCompact:
		tp.TProtocol = NewTCompactProtocol(tp.trans)
	default:
		panic(NewTProtocolException(fmt.Errorf("Unknown protocol id: %#x", tp.protoID)))
	}
	return tp
}

func NewHeaderProtocol(trans TTransport) *HeaderProtocol {
	p := &HeaderProtocol{
		origTransport: trans,
		protoID:       ProtocolIDBinary,
	}
	if et, ok := trans.(*HeaderTransport); ok {
		p.trans = et
	} else {
		p.trans = NewHeaderTransport(trans)
	}

	// Effectively an invariant violation.
	if err := p.ResetProtocol(); err != nil {
		panic(err)
	}
	return p
}

func (p *HeaderProtocol) ResetProtocol() error {
	p.trans.SetProtocolID(p.protoID)
	if p.TProtocol != nil {
		return nil
	}

	switch p.protoID {
	case ProtocolIDBinary:
		// These defaults match cpp implementation
		p.TProtocol = NewTBinaryProtocol(p.trans, false, true)
	case ProtocolIDCompact:
		p.TProtocol = NewTCompactProtocol(p.trans)
	default:
		return NewTProtocolException(fmt.Errorf("Unknown protocol id: %#x", p.protoID))
	}
	return nil
}

//
// Writing methods.
//

func (p *HeaderProtocol) WriteMessageBegin(name string, typeId TMessageType, seqid int32) error {
	p.ResetProtocol()
	// only now we know the msg type
	p.trans.SetIntHeader(INT_HEADER_MSG_TYPE, strconv.Itoa(int(typeId)))
	// FIXME: Python is doing this -- don't know if it's correct.
	// Should we be using this seqid or the header's?
	if typeId == CALL || typeId == ONEWAY {
		p.trans.SetSeqID(uint32(seqid))
	}
	return p.TProtocol.WriteMessageBegin(name, typeId, seqid)
}

//
// Reading methods.
//

func (p *HeaderProtocol) ReadMessageBegin() (name string, typeId TMessageType, seqid int32, err error) {
	if typeId == INVALID_TMESSAGE_TYPE {
		if err = p.trans.ResetProtocol(); err != nil {
			return name, EXCEPTION, seqid, err
		}
	}

	err = p.ResetProtocol()
	if err != nil {
		return name, EXCEPTION, seqid, err
	}

	return p.TProtocol.ReadMessageBegin()
}

func (p *HeaderProtocol) Flush() (err error) {
	return NewTProtocolException(p.trans.Flush())
}

func (p *HeaderProtocol) Skip(fieldType TType) (err error) {
	return SkipDefaultDepth(p, fieldType)
}

func (p *HeaderProtocol) Transport() TTransport {
	return p.origTransport
}

func (p *HeaderProtocol) HeaderTransport() TTransport {
	return p.trans
}

// Control underlying header transport

func (p *HeaderProtocol) SetIdentity(identity string) {
	p.trans.SetIdentity(identity)
}

func (p *HeaderProtocol) Identity() string {
	return p.trans.Identity()
}

func (p *HeaderProtocol) PeerIdentity() string {
	return p.trans.PeerIdentity()
}

func (p *HeaderProtocol) SetHeaders(headers map[string]string) {
	p.trans.SetHeaders(headers)
}

func (p *HeaderProtocol) SetHeader(key, value string) {
	p.trans.SetHeader(key, value)
}

func (p *HeaderProtocol) Header(key string) (string, bool) {
	return p.trans.Header(key)
}

func (p *HeaderProtocol) Headers() map[string]string {
	return p.trans.Headers()
}

func (p *HeaderProtocol) ClearHeaders() {
	p.trans.ClearHeaders()
}

func (p *HeaderProtocol) ReadHeader(key string) (string, bool) {
	return p.trans.ReadHeader(key)
}

func (p *HeaderProtocol) ReadHeaders() map[string]string {
	return p.trans.ReadHeaders()
}

func (p *HeaderProtocol) ProtocolID() ProtocolID {
	return p.protoID
}

func (p *HeaderProtocol) AddTransform(trans TransformID) error {
	return p.trans.AddTransform(trans)
}
