package kitc

import (
	"bytes"
	"context"
	"sync"

	"code.byted.org/gopkg/thrift"
)

// MockBufferedTransport implement Transport
type MockBufferedTransport struct {
	in          *bytes.Buffer
	out         *bytes.Buffer
	processFunc ProcessFunc
	once        sync.Once
}

type ProcessFunc func(transport *MockBufferedTransport) (err thrift.TException)

// NewMockTransport return a MockBufferedTransport
func NewMockBufferedTransport(f ProcessFunc) Transport {
	return &MockBufferedTransport{in: &bytes.Buffer{}, processFunc: f}
}

func (mt *MockBufferedTransport) Process() (err thrift.TException) {
	if mt.processFunc == nil {
		return
	}

	mt.once.Do(func() {
		if mt.out == nil {
			mt.out = &bytes.Buffer{}
		}
		mt.out.Reset()

		err = mt.processFunc(&MockBufferedTransport{in: mt.out, out: mt.in})
	})
	return
}

func (mt *MockBufferedTransport) RemoteAddr() string {
	return ""
}

func (mt *MockBufferedTransport) Read(p []byte) (int, error) {
	err := mt.Process()
	if err != nil {
		return 0, err
	}
	return mt.out.Read(p)
}

func (mt *MockBufferedTransport) ReadByte() (byte, error) {
	err := mt.Process()
	if err != nil {
		return 0, err
	}
	return mt.out.ReadByte()
}

func (mt *MockBufferedTransport) Write(p []byte) (int, error) {
	return mt.in.Write(p)
}

func (mt *MockBufferedTransport) WriteByte(c byte) error {
	return mt.in.WriteByte(c)
}

func (mt *MockBufferedTransport) WriteString(s string) (int, error) {
	return mt.in.WriteString(s)
}

func (mt *MockBufferedTransport) Flush() error {
	return nil
}

func (mt *MockBufferedTransport) Open() error {
	return nil
}

func (mt *MockBufferedTransport) OpenWithContext(ctx context.Context) error {
	return nil
}

func (mt *MockBufferedTransport) IsOpen() bool {
	return true
}

func (mt *MockBufferedTransport) Close() error {
	mt.in.Reset()
	mt.out.Reset()
	return nil
}
