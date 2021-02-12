package thrift

import (
	"io"
)

/*至少确保有1个元素*/
type MultiIoBuffer struct {
	bufs []*IoBuffer
}

func NewMultiIoBuffer(size int) *MultiIoBuffer {
	return &MultiIoBuffer{bufs: []*IoBuffer{GetIoBuffer(size)}}
}

func (b *MultiIoBuffer) Reset() {
	for idx := 1; idx < len(b.bufs); idx++ {
		_ = PutIoBuffer(b.bufs[idx])
	}
	b.bufs[0].Reset()
	b.bufs = b.bufs[:1]
}

func (b *MultiIoBuffer) Free() {
	for _, buf := range b.bufs {
		_ = PutIoBuffer(buf)
	}
	b.bufs = b.bufs[:0]
}

func (b *MultiIoBuffer) Write(data []byte) (int, error) {
	buf := b.bufs[len(b.bufs)-1]
	if !buf.IsWriteable(len(data)) {
		pos := buf.AvailableWrite()
		_, _ = buf.Write(data[:pos])
		buf = NewIoBuffer(0)
		b.bufs = append(b.bufs, buf)
		data = data[pos:]
	}
	return buf.Write(data)
}

func (b *MultiIoBuffer) WriteString(s string) (int, error) {
	buf := b.bufs[len(b.bufs)-1]
	if !buf.IsWriteable(len(s)) {
		pos := buf.AvailableWrite()
		_, _ = buf.WriteString(s[:pos])
		buf = NewIoBuffer(0)
		b.bufs = append(b.bufs, buf)
		s = s[pos:]
	}
	return buf.WriteString(s)
}

/*一般一次请求只用一次，不单独使用一个成员变量累加*/
func (b *MultiIoBuffer) Len() int {
	ret := 0
	for _, buf := range b.bufs {
		ret += buf.Len()
	}
	return ret
}

func (b *MultiIoBuffer) WriteTo(bw io.Writer) (err error) {
	for _, buf := range b.bufs {
		if buf.Len() > 0 {
			if buf.Len() > FlushBatchSize {
				bw = NewBatchWriter(bw, FlushBatchSize)
			}
			
			_, err = buf.WriteTo(bw)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *MultiIoBuffer) Merge() *IoBuffer {
	if len(b.bufs) > 1 {
		ret := NewIoBuffer(b.Len())
		for _, buf := range b.bufs {
			_, _ = ret.Write(buf.Bytes())
		}
		return ret
	}
	return b.bufs[0]
}

func (b *MultiIoBuffer) BufferNum() int {
	return len(b.bufs)
}