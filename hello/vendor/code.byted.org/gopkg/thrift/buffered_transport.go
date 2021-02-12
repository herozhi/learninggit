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
	"io"
	"sync"
)

var (
	bufioPoolrw sync.Pool
)

func newBufioRW(r io.Reader, w io.Writer, size int) *bufio.ReadWriter {
	if v := bufioPoolrw.Get(); v != nil {
		brw := v.(*bufio.ReadWriter)
		brw.Reader.Reset(r)
		brw.Writer.Reset(w)
		return brw
	}
	return bufio.NewReadWriter(bufio.NewReaderSize(r, size), bufio.NewWriterSize(w, size))
}

func putBufioRW(rw *bufio.ReadWriter) {
	rw.Reader.Reset(nil)
	rw.Writer.Reset(nil)
	bufioPoolrw.Put(rw)
}

type TBufferedTransportFactory struct {
	size int
}

type TBufferedTransport struct {
	bufio.ReadWriter
	tp TTransport
}

func (p *TBufferedTransportFactory) GetTransport(trans TTransport) TTransport {
	return NewTBufferedTransport(trans, p.size)
}

func NewTBufferedTransportFactory(bufferSize int) *TBufferedTransportFactory {
	return &TBufferedTransportFactory{size: bufferSize}
}

func NewTBufferedTransport(trans TTransport, bufferSize int) *TBufferedTransport {
	return &TBufferedTransport{
		ReadWriter: *newBufioRW(trans, trans, bufferSize),
		tp:         trans,
	}
}

func (p *TBufferedTransport) IsOpen() bool {
	return p.tp.IsOpen()
}

func (p *TBufferedTransport) Open() (err error) {
	return p.tp.Open()
}

func (p *TBufferedTransport) Close() (err error) {
	putBufioRW(&p.ReadWriter)
	return p.tp.Close()
}

func (p *TBufferedTransport) Flush() error {
	if err := p.ReadWriter.Flush(); err != nil {
		return err
	}
	return p.tp.Flush()
}
