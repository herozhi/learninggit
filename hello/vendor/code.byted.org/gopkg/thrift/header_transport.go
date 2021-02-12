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
	"bufio"
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"math"
)

const (
	DefaultProtoID     = ProtocolIDBinary
	DefaultClientType = UnframedDeprecated //为了极大的兼容旧协议和旧接口，设置默认为unframed
	DefaultBufSize    = 4096
)

type tHeaderTransportFactory struct {
	factory TTransportFactory
}

func NewHeaderTransportFactory(factory TTransportFactory) TTransportFactory {
	return &tHeaderTransportFactory{factory: factory}
}

func (p *tHeaderTransportFactory) GetTransport(base TTransport) TTransport {
	return NewHeaderTransport(p.factory.GetTransport(base))
}

type HeaderTransport struct {
	transport TTransport

	// Used on read
	rbuf       *bufio.Reader
	framebuf   byteReader
	readHeader *tHeader
	// remaining bytes in the current frame. If 0, read in a new frame.
	frameSize uint64

	// Used on write
	//wbuf                *IoBuffer
	/*
	 wbufs替代wbuf;
	 buffered、frame、header协议下，wbufs长度>=1，由多个wbuf拼接成一个总的wbuf，避免大段内存拷贝
	*/
	wbufs               *MultiIoBuffer
	
	identity            string
	writeInfoHeaders    map[string]string
	writeInfoIntHeaders map[uint16]string

	// Negotiated
	protoID         ProtocolID
	seqID           uint32
	flags           uint16
	clientType      ClientType
	writeTransforms []TransformID
}

// NewHeaderTransport Create a new transport with defaults.
func NewHeaderTransport(transport TTransport) *HeaderTransport {
	hdrTrans := &HeaderTransport{
		framebuf:  newLimitedByteReader(bytes.NewReader(nil), 0),
		frameSize: 0,

		writeInfoHeaders:    map[string]string{},
		writeInfoIntHeaders: map[uint16]string{},

		protoID:         DefaultProtoID,
		flags:           0,
		writeTransforms: []TransformID{},
	}

	// TODO:
	//  In most cases, the transport is TBufferedTransport,
	//  and for performance we might need the underlying socket transport
	switch t := transport.(type) {
	case *TBufferedTransport, *TMemoryBuffer:
		hdrTrans.transport = transport
		hdrTrans.clientType = UnframedDeprecated
	case *TFramedTransport:
		hdrTrans.transport = t.transport
		hdrTrans.clientType = RawFramedDeprecated
	default:
		hdrTrans.transport = transport
		hdrTrans.clientType = DefaultClientType
	}
	//hdrTrans.rbuf = bufio.NewReader(hdrTrans.transport)
	// 由于Reader没有做缓存，这里限制Reader size大小，避免申请过多无效内存
	size := defaultReadSize
	if size > MaxBufferLen {
		size = MaxBufferLen
	}
	hdrTrans.rbuf = bufio.NewReaderSize(hdrTrans.transport, size)

	hdrTrans.wbufs = NewMultiIoBuffer(0)
	return hdrTrans
}

func (t *HeaderTransport) IsNil() bool {
	return t == nil
}

// SetClientType. 为了极大兼容旧协议和接口的各种情况，所有client使用TTHeader协议，都必须显示设置
func (t *HeaderTransport) SetClientType(clientType ClientType) {
	t.clientType = clientType
}

func (t *HeaderTransport) SetSeqID(seq uint32) {
	t.seqID = seq
}

func (t *HeaderTransport) SeqID() uint32 {
	return t.seqID
}

func (t *HeaderTransport) Identity() string {
	return t.identity
}

func (t *HeaderTransport) SetIdentity(identity string) {
	t.identity = identity
}

func (t *HeaderTransport) PeerIdentity() string {
	v, ok := t.ReadHeader(IdentityHeader)
	vers, versok := t.ReadHeader(IDVersionHeader)
	if ok && versok && vers == IDVersion {
		return v
	}
	return ""
}

func (t *HeaderTransport) SetHeaders(headers map[string]string) {
	t.writeInfoHeaders = headers
}

func (t *HeaderTransport) SetHeader(key, value string) {
	t.writeInfoHeaders[key] = value
}

func (t *HeaderTransport) Header(key string) (string, bool) {
	v, ok := t.writeInfoHeaders[key]
	return v, ok
}

func (t *HeaderTransport) Headers() map[string]string {
	res := map[string]string{}
	for k, v := range t.writeInfoHeaders {
		res[k] = v
	}
	return res
}

func (t *HeaderTransport) ClearHeaders() {
	t.writeInfoHeaders = map[string]string{}
}

func (t *HeaderTransport) SetIntHeader(key uint16, value string) {
	t.writeInfoIntHeaders[key] = value
}

func (t *HeaderTransport) SetIntHeaders(headers map[uint16]string) {
	t.writeInfoIntHeaders = headers
}

func (t *HeaderTransport) IntHeader(key uint16) (string, bool) {
	v, ok := t.writeInfoIntHeaders[key]
	return v, ok
}

func (t *HeaderTransport) IntHeaders() map[uint16]string {
	res := map[uint16]string{}
	for k, v := range t.writeInfoIntHeaders {
		res[k] = v
	}
	return res
}

func (t *HeaderTransport) ClearIntHeaders() {
	t.writeInfoIntHeaders = map[uint16]string{}
}

func (t *HeaderTransport) ReadHeader(key string) (string, bool) {
	if t.readHeader == nil {
		return "", false
	}
	v, ok := t.readHeader.headers[key]
	return v, ok
}

func (t *HeaderTransport) ReadHeaders() map[string]string {
	res := map[string]string{}
	if t.readHeader == nil {
		return res
	}
	for k, v := range t.readHeader.headers {
		res[k] = v
	}
	return res
}

func (t *HeaderTransport) ReadIntHeader(key uint16) (string, bool) {
	if t.readHeader == nil {
		return "", false
	}
	v, ok := t.readHeader.intHeaders[key]
	return v, ok
}

func (t *HeaderTransport) ReadIntHeaders() map[uint16]string {
	res := map[uint16]string{}
	if t.readHeader == nil {
		return res
	}
	for k, v := range t.readHeader.intHeaders {
		res[k] = v
	}
	return res
}

func (t *HeaderTransport) ProtocolID() ProtocolID {
	return t.protoID
}

func (t *HeaderTransport) SetProtocolID(protoID ProtocolID) error {
	if !(protoID == ProtocolIDBinary || protoID == ProtocolIDCompact) {
		return NewTTransportException(
			NOT_IMPLEMENTED, fmt.Sprintf("unimplemented proto ID: %s (%#x)", protoID.String(), int64(protoID)),
		)
	}
	t.protoID = protoID
	return nil
}

func (t *HeaderTransport) AddTransform(trans TransformID) error {
	if sup, ok := supportedTransforms[trans]; !ok || !sup {
		return NewTTransportException(
			NOT_IMPLEMENTED, fmt.Sprintf("unimplemented transform ID: %s (%#x)", trans.String(), int64(trans)),
		)
	}
	for _, t := range t.writeTransforms {
		if t == trans {
			return nil
		}
	}
	t.writeTransforms = append(t.writeTransforms, trans)
	return nil
}

// applyUntransform Fully read the frame and untransform into a local buffer
// we need to know the full size of the untransformed data
func (t *HeaderTransport) applyUntransform() error {
	out, err := ioutil.ReadAll(t.framebuf)
	if err != nil {
		return err
	}
	t.frameSize = uint64(len(out))
	t.framebuf = newLimitedByteReader(bytes.NewBuffer(out), int64(len(out)))
	return nil
}

// ResetProtocol Needs to be called between every frame receive (BeginMessageRead)
// We do this to read out the header for each frame. This contains the length of the
// frame and protocol / metadata info.
func (t *HeaderTransport) ResetProtocol() error {
	t.readHeader = nil
	// TODO(carlverge): We should probably just read in the whole
	// frame here. A bit of extra memory, probably a lot less CPU.
	// Needs benchmark to test.

	hdr := &tHeader{}
	// Consume the header from the input stream
	err := hdr.Read(t.rbuf)
	if err != nil {
		return NewTTransportExceptionFromError(err)
	}

	// Set new header
	t.readHeader = hdr
	// Adopt the client's protocol
	t.protoID = hdr.protoID
	t.clientType = hdr.clientType
	t.seqID = hdr.seq
	t.flags = hdr.flags

	// If the client is using unframed, just pass up the data to the protocol
	if t.clientType == UnframedBinaryDeprecated || t.clientType == UnframedCompactDeprecated || t.clientType == UnframedDeprecated {
		t.framebuf = t.rbuf
		return nil
	}

	// Make sure we can't read past the current frame length
	t.frameSize = hdr.payloadLen
	t.framebuf = newLimitedByteReader(t.rbuf, int64(hdr.payloadLen))

	for _, trans := range hdr.transforms {
		xformer, terr := trans.Untransformer()
		if terr != nil {
			return NewTTransportExceptionFromError(terr)
		}

		t.framebuf, terr = xformer(t.framebuf)
		if terr != nil {
			return NewTTransportExceptionFromError(terr)
		}
	}

	// Fully read the frame and apply untransforms if we have them
	if len(hdr.transforms) > 0 {
		err = t.applyUntransform()
		if err != nil {
			return NewTTransportExceptionFromError(err)
		}
	}

	// respond in kind with the client's transforms
	t.writeTransforms = hdr.transforms

	return nil
}

// Open Open the internal transport
func (t *HeaderTransport) Open() error {
	return t.transport.Open()
}

// IsOpen Is the current transport open
func (t *HeaderTransport) IsOpen() bool {
	return t.transport.IsOpen()
}

// Close Close the internal transport
func (t *HeaderTransport) Close() error {
	t.wbufs.Free()
	return t.transport.Close()
}

// Read Read from the current framebuffer. EOF if the frame is done.
func (t *HeaderTransport) Read(buf []byte) (int, error) {
	// If we detected unframed, just pass the transport up
	if t.clientType == UnframedBinaryDeprecated || t.clientType == UnframedCompactDeprecated || t.clientType == UnframedDeprecated {
		return t.framebuf.Read(buf)
	}
	n, err := t.framebuf.Read(buf)
	// Shouldn't be possibe, but just in case the frame size was flubbed
	if uint64(n) > t.frameSize {
		n = int(t.frameSize)
	}
	t.frameSize -= uint64(n)
	return n, err
}

// ReadByte Read a single byte from the current framebuffer. EOF if the frame is done.
func (t *HeaderTransport) ReadByte() (byte, error) {
	// If we detected unframed, just pass the transport up
	if t.clientType == UnframedBinaryDeprecated || t.clientType == UnframedCompactDeprecated || t.clientType == UnframedDeprecated {
		return t.framebuf.ReadByte()
	}
	b, err := t.framebuf.ReadByte()
	t.frameSize--
	return b, err
}

// Write Write multiple bytes to the framebuffer, does not send to transport.
func (t *HeaderTransport) Write(buf []byte) (n int, err error) {
	return t.wbufs.Write(buf)
}

// WriteByte Write a single byte to the framebuffer, does not send to transport.
func (t *HeaderTransport) WriteByte(c byte) (err error) {
	tmp := [1]byte{c}
	_, err =  t.wbufs.Write(tmp[:])
	return
}

// WriteString Write a string to the framebuffer, does not send to transport.
func (t *HeaderTransport) WriteString(s string) (n int, err error) {
	return t.wbufs.WriteString(s)
}

// RemainingBytes Return how many bytes remain in the current recv framebuffer.
func (t *HeaderTransport) RemainingBytes() uint64 {
	if t.clientType == UnframedBinaryDeprecated || t.clientType == UnframedCompactDeprecated || t.clientType == UnframedDeprecated {
		// We cannot really tell the size without reading the whole struct in here
		return math.MaxUint64
	}
	return t.frameSize
}

func applyTransforms(buf *IoBuffer, transforms []TransformID) (*IoBuffer, error) {
	if len(transforms) == 0 {
		return buf, nil
	}
	tmpbuf := GetIoBuffer(buf.Len())
	defer PutIoBuffer(tmpbuf)
	for _, trans := range transforms {
		switch trans {
		case TransformZlib:
			zwr := zlib.NewWriter(tmpbuf)
			_, err := buf.WriteTo(zwr)
			if err != nil {
				return nil, err
			}
			err = zwr.Close()
			if err != nil {
				return nil, err
			}
			buf, tmpbuf = tmpbuf, buf
			tmpbuf.Reset()
		default:
			return nil, NewTTransportException(
				NOT_IMPLEMENTED, fmt.Sprintf("unimplemented transform ID: %s (%#x)", trans.String(), int64(trans)),
			)
		}
	}
	return buf, nil
}

func (t *HeaderTransport) flushHeader() error {
	hdr := tHeader{}
	hdr.headers = t.writeInfoHeaders
	hdr.intHeaders = t.writeInfoIntHeaders
	hdr.protoID = t.protoID
	hdr.clientType = t.clientType
	hdr.seq = t.seqID
	hdr.flags = t.flags
	hdr.transforms = t.writeTransforms

	if t.identity != "" {
		hdr.headers[IdentityHeader] = t.identity
		hdr.headers[IDVersionHeader] = IDVersion
	}
	
	var err error
	totalLen := t.wbufs.Len()
	if len(t.writeTransforms) != 0 {
		wbuf := t.wbufs.Merge()
		outbuf, err := applyTransforms(wbuf, t.writeTransforms)
		if err != nil {
			return NewTTransportExceptionFromError(err)
		}
		if t.wbufs.BufferNum() > 1 {
			_ = PutIoBuffer(wbuf)
		}
		t.wbufs.Free()
		t.wbufs = &MultiIoBuffer{bufs: []*IoBuffer{outbuf}}
		totalLen = outbuf.Len()
	}

	hdr.payloadLen = uint64(totalLen)
	err = hdr.calcLenFromPayload()
	if err != nil {
		return NewTTransportExceptionFromError(err)
	}

	err = hdr.Write(t.transport)
	if err != nil {
		return NewTTransportExceptionFromError(err)
	}

	// write inner frame size
	if t.clientType == TTHeaderFramedClientType {
		if hdr.payloadLen > uint64(MaxFrameSize) {
			return NewTTransportException(
				INVALID_FRAME_SIZE,
				fmt.Sprintf("cannot send bigframe of size %d", hdr.payloadLen),
			)
		}
		var buf [4]byte
		binary.BigEndian.PutUint32(buf[:4], uint32(hdr.payloadLen))
		if _, err := t.transport.Write(buf[:4]); err != nil {
			return NewTTransportExceptionFromError(err)
		}
	}

	return nil
}

func (t *HeaderTransport) flushFramed(withFrameSize bool) error {
	if !withFrameSize {
		return nil
	}
	framesize := uint32(t.wbufs.Len())
	if framesize > MaxFrameSize {
		return NewTTransportException(
			INVALID_FRAME_SIZE,
			fmt.Sprintf("cannot send bigframe of size %d", framesize),
		)
	}

	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:4], framesize)
	if _, err := t.transport.Write(buf[:4]); err != nil {
		return NewTTransportExceptionFromError(err)
	}
	return nil
}

func (t *HeaderTransport) Flush() error {
	// Closure incase wbuf pointer changes in xform
	defer func(tp *HeaderTransport) {
		tp.wbufs.Reset()
	}(t)
	var err error
	switch t.clientType {
	case TTHeaderUnframedClientType, TTHeaderFramedClientType:
		err = t.flushHeader()
	case RawFramedDeprecated, FramedDeprecated, FramedBinaryDeprecated, FramedCompactDeprecated:
		err = t.flushFramed(true)
	case UnframedDeprecated, UnframedCompactDeprecated, UnframedBinaryDeprecated:
		err = nil
	default:
		return NewTTransportException(
			UNKNOWN_TRANSPORT_EXCEPTION,
			fmt.Sprintf("tHeader cannot flush for clientType %s", t.clientType.String()),
		)
	}
	if err != nil {
		return err
	}
	
	// Writeout the payload
	err = t.wbufs.WriteTo(t.transport)
	if err != nil {
		return NewTTransportExceptionFromError(err)
	}
	
	// Remove the non-persistent headers on flush
	t.ClearHeaders()
	t.ClearIntHeaders()
	
	err = t.transport.Flush()
	if err == nil {
		return nil
	}
	return NewTTransportExceptionFromError(err)
}

// ResetFramebuf  make serializer happy
func (t *HeaderTransport) ResetFramebuf(r byteReader) {
	t.framebuf = r
}
