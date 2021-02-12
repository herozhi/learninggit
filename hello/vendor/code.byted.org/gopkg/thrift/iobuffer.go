/*
* Use one byte[] for iobuffer, can read and write
 */

package thrift

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"
	"unsafe"
)

const (
	MinRead        = 1 << 9
	MaxRead        = 1 << 17
	ResetOffMark   = -1
	FlushBatchSize = 8192 << 4
	MaxBufferLen   = 8192 << 1
)

var defaultSize = 1 << 12
var defaultReadSize = 1 << 12
var networkBusyLimit = false

func SetIOBufferDefaultSize(size int) {
	if size <= 0 || size > (1 << 20) {
		size = 1 << 12
	}
	defaultSize = size
}

func SetDefaultReadSize(size int) {
	if size <= 0 || size > (1 << 20) {
		size = 1 << 12
	}
	defaultReadSize = size
}

// 重io读写io次数过多导致服务雪崩，开启之后能避免雪崩；可能会带来部分io请求失败
func SetNetworkBusyLimit(flag bool) {
	networkBusyLimit = flag
}

func NewBatchWriter(w io.Writer, size int) io.Writer {
	if size <= 0 {
		panic(fmt.Sprintf("invalid batch size: %d", size))
	}
	return &batchWriter{w, size}
}

type batchWriter struct {
	io.Writer
	size int
}

func (bw *batchWriter) Write(p []byte) (n int, err error) {
	var m, b, e int
	// 网卡满的时候，经常写不出数据，产生写几千几万次；system call严重；这里做次数限制
	c, maxCount := uint64(0), (uint64(len(p)) >> 15) + 64
	l := len(p)
	for b < l {
		if e = b + bw.size; e > l {
			e = l
		}
		if n, err = bw.Writer.Write(p[b:e]); err != nil {
			return m + n, err
		}
		m += n
		b += bw.size
		c++
		if networkBusyLimit && c >= maxCount {
			return m, ErrNetworkBusy
		}
	}
	return m, nil
}

var nullByte []byte

var (
	EOF                  = errors.New("EOF")
	ErrTooLarge          = errors.New("io buffer: too large")
	ErrNegativeCount     = errors.New("io buffer: negative count")
	ErrInvalidWriteCount = errors.New("io buffer: invalid write count")
	ErrInvalidReadCount  = errors.New("io buffer: invalid read count")
	ErrNetworkBusy       = errors.New("batchWriter: network busy")
)

// IoBuffer
type IoBuffer struct {
	buf     []byte // contents: buf[off : len(buf)]
	off     int    // read from &buf[off], write to &buf[len(buf)]
	offMark int
	count   int32
	eof     bool
	b       *[]byte
}

/*
 大部分读ReadI16, ReadI32, ReadI64
*/
func (b *IoBuffer) Read(p []byte) (n int, err error) {
	lp := len(p)
	if lp == 0 {
		return 0, nil
	}
	lb := len(b.buf)
	if b.off >= lb {
		b.Reset()
		return 0, io.EOF
	}
	n = lp
	if lb - b.off >= lp {
		ptr := unsafe.Pointer(&p[0])
		ptr1 := unsafe.Pointer(&b.buf[b.off])
		switch lp {
		case 1:
			*(*byte)(ptr) = *(*byte)(ptr1)
		case 2:
			*(*uint16)(ptr) = *(*uint16)(ptr1)
		case 4:
			*(*uint32)(ptr) = *(*uint32)(ptr1)
		case 8:
			*(*uint64)(ptr) = *(*uint64)(ptr1)
		default:
			n = copy(p, b.buf[b.off:])
		}
	} else {
		n = copy(p, b.buf[b.off:])
	}
	b.off += n
	return
}

func (b *IoBuffer) ReadByte() (bt byte, err error) {
	if b.off >= len(b.buf) {
		b.Reset()
		return 0, io.EOF
	}
	bt = b.buf[b.off]
	b.off += 1
	return
}

func (b *IoBuffer) ReadOnce(r io.Reader, timeout time.Duration) (n int, err error) {
	var (
		m               int
		e               error
		conn            net.Conn
		loop, ok, first = true, true, true
	)

	if conn, ok = r.(net.Conn); !ok {
		loop = false
	}

	if b.off >= len(b.buf) {
		b.Reset()
	}

	if b.off > 0 && len(b.buf)-b.off < 4*MinRead {
		b.copy(0)
	}

	for {
		if first == false {
			if free := cap(b.buf) - len(b.buf); free < MinRead {
				// not enough space at end
				if b.off+free < MinRead {
					// not enough space using beginning of buffer;
					// double buffer capacity
					b.copy(MinRead)
				} else {
					b.copy(0)
				}
			}
		}

		l := cap(b.buf) - len(b.buf)

		if conn != nil {
			conn.SetReadDeadline(time.Now().Add(timeout))
			m, e = r.Read(b.buf[len(b.buf):cap(b.buf)])
		} else {
			m, e = r.Read(b.buf[len(b.buf):cap(b.buf)])
		}

		if m > 0 {
			b.buf = b.buf[0 : len(b.buf)+m]
			n += m
		}

		if e != nil {
			if te, ok := err.(net.Error); ok && te.Timeout() && !first {
				return n, nil
			}
			return n, e
		}

		if l != m {
			loop = false
		}

		if n > MaxRead {
			loop = false
		}

		if !loop {
			break
		}

		first = false
	}

	return n, nil
}

func (b *IoBuffer) ReadFrom(r io.Reader) (n int64, err error) {
	if b.off >= len(b.buf) {
		b.Reset()
	}
	
	var free = cap(b.buf) - len(b.buf)
	// 网卡满的时候，经常读不到数据，产生读几千几万次；system call严重；这里做次数限制
	var maxCount, c = (uint64(free) >> 15) + 64, uint64(0)

	for {
		if free := cap(b.buf) - len(b.buf); free < MinRead {
			// not enough space at end
			if b.off+free < MinRead {
				// not enough space using beginning of buffer;
				// double buffer capacity
				b.copy(MinRead)
			} else {
				b.copy(0)
			}
		}

		m, e := r.Read(b.buf[len(b.buf):cap(b.buf)])

		b.buf = b.buf[0 : len(b.buf)+m]
		n += int64(m)

		if e == io.EOF {
			break
		}

		if m == 0 {
			break
		}

		if e != nil {
			return n, e
		}
		c++
		if networkBusyLimit && c >= maxCount {
			return n, ErrNetworkBusy
		}
	}

	return
}

/*
  绝大部分都是writeByte, writeI16, writeI32, writeI64；这里直接用unsafe.Pointer实现，效率提升2倍
*/
func (b *IoBuffer) Write(p []byte) (n int, err error) {
	l := len(p)
	if l == 0 {
		return
	}
	m, ok := b.tryGrowByReslice(l)
	if !ok {
		m = b.Grow(l)
	}
	ptr := unsafe.Pointer(&b.buf[m])
	ptr1 := unsafe.Pointer(&p[0])
	switch l {
	case 1:
		*(*byte)(ptr) = *(*byte)(ptr1)
	case 2:
		*(*uint16)(ptr) = *(*uint16)(ptr1)
	case 4:
		*(*uint32)(ptr) = *(*uint32)(ptr1)
	case 8:
		*(*uint64)(ptr) = *(*uint64)(ptr1)
	default:
		copy(b.buf[m:], p)
	}
	return l, nil
}

func (b *IoBuffer) WriteString(s string) (n int, err error) {
	l := len(s)
	if l == 0 {
		return
	}
	m, ok := b.tryGrowByReslice(l)
	if !ok {
		m = b.Grow(l)
	}
	return copy(b.buf[m:], s), nil
}

func (b *IoBuffer) tryGrowByReslice(n int) (int, bool) {
	if l := len(b.buf); l+n <= cap(b.buf) {
		b.buf = b.buf[:l+n]
		return l, true
	}

	return 0, false
}

func (b *IoBuffer) Grow(n int) int {
	m := b.Len()
	// If buffer is empty, reset to recover space.
	if m == 0 && b.off != 0 {
		b.Reset()
	}

	// Try to grow by means of a reslice.
	if i, ok := b.tryGrowByReslice(n); ok {
		return i
	}
	// 原有逻辑是一次性写入transport，提前按1/2因子扩容有效
	// growLimit := cap(b.buf) >> 1
	// 新逻辑会分段写入tranport，无需设置因子，直接写满
	growLimit := cap(b.buf)
	if m+n <= growLimit {
		// We can slide things down instead of allocating a new
		// slice. We only need m+n <= cap(b.buf) to slide, but
		// we instead let capacity get twice as large so we
		// don't spend all our time copying.
		b.copy(0)
	} else {
		// Not enough space anywhere, we need to allocate.
		b.copy(n)
	}

	// Restore b.off and len(b.buf).
	b.off = 0
	b.buf = b.buf[:m+n]

	return m
}

func (b *IoBuffer) WriteTo(w io.Writer) (n int64, err error) {
	// 网卡满的时候，经常读不到数据，产生读几千几万次；system call严重；这里做次数限制
	var maxCount, c = (uint64(b.Len()) >> 15) + 64, uint64(0)
	for b.off < len(b.buf) {
		nBytes := b.Len()
		m, e := w.Write(b.buf[b.off:])

		if m > nBytes {
			panic(ErrInvalidWriteCount)
		}

		b.off += m
		n += int64(m)

		if e != nil {
			return n, e
		}
		
		c++
		if networkBusyLimit && c >= maxCount {
			return n, ErrNetworkBusy
		}

		if m == 0 || m == nBytes {
			return n, nil
		}
	}

	return
}

func (b *IoBuffer) AppendBytes(data []byte) error {
	if b.off >= len(b.buf) {
		b.Reset()
	}

	dataLen := len(data)

	if free := cap(b.buf) - len(b.buf); free < dataLen {
		// not enough space at end
		if b.off+free < dataLen {
			// not enough space using beginning of buffer;
			// double buffer capacity
			b.copy(dataLen)
		} else {
			b.copy(0)
		}
	}

	m := copy(b.buf[len(b.buf):len(b.buf)+dataLen], data)
	b.buf = b.buf[0 : len(b.buf)+m]

	return nil
}

func (b *IoBuffer) AppendByte(data byte) error {
	return b.AppendBytes([]byte{data})
}

func (b *IoBuffer) Peek(n int) []byte {
	if len(b.buf)-b.off < n {
		return nil
	}

	return b.buf[b.off : b.off+n]
}

func (b *IoBuffer) Mark() {
	b.offMark = b.off
}

func (b *IoBuffer) Restore() {
	if b.offMark != ResetOffMark {
		b.off = b.offMark
		b.offMark = ResetOffMark
	}
}

func (b *IoBuffer) Bytes() []byte {
	return b.buf[b.off:]
}

func (b *IoBuffer) Cut(offset int) *IoBuffer {
	if b.off+offset > len(b.buf) {
		return nil
	}

	buf := make([]byte, offset)

	copy(buf, b.buf[b.off:b.off+offset])
	b.off += offset
	b.offMark = ResetOffMark

	return &IoBuffer{
		buf: buf,
		off: 0,
	}
}

func (b *IoBuffer) Drain(offset int) {
	if b.off+offset > len(b.buf) {
		return
	}

	b.off += offset
	b.offMark = ResetOffMark
}

func (b *IoBuffer) String() string {
	return string(b.buf[b.off:])
}

func (b *IoBuffer) Len() int {
	return len(b.buf) - b.off
}

func (b *IoBuffer) Cap() int {
	return cap(b.buf)
}

func (b *IoBuffer) Reset() {
	b.buf = b.buf[:0]
	b.off = 0
	b.offMark = ResetOffMark
	b.eof = false
}

func (b *IoBuffer) Free() {
	b.Reset()
	b.giveSlice()
}

func (b *IoBuffer) makeSlice(n int) *[]byte {
	return GetBytes(n)
}

func (b *IoBuffer) giveSlice() {
	if b.b != nil {
		PutBytes(b.b)
		b.b = nil
		b.buf = nullByte
	}
}

func (b *IoBuffer) Flush() {
	b.Reset()
}

func (b *IoBuffer) AvailableRead() int {
	return len(b.buf) - b.off
}

func (b *IoBuffer) AvailableWrite() int {
	return cap(b.buf) - len(b.buf)
}

// 判断iobuffer是否有足够空间继续写内容
func (b *IoBuffer) IsWriteable(n int) bool {
	return b.Len() < MaxBufferLen || b.AvailableWrite() >= n
}

func (b *IoBuffer) Clone() *IoBuffer {
	buf := GetIoBuffer(b.Len())
	buf.Write(b.Bytes())

	buf.SetEOF(b.EOF())

	return buf
}

func (b *IoBuffer) Alloc(size int) {
	if b.buf != nil {
		b.Free()
	}
	if size <= 0 {
		size = defaultSize
	}

	b.b = b.makeSlice(size)
	b.buf = *b.b
	b.buf = b.buf[:0]
}

func (b *IoBuffer) Count(count int32) int32 {
	return atomic.AddInt32(&b.count, count)
}

func (b *IoBuffer) EOF() bool {
	return b.eof
}

func (b *IoBuffer) SetEOF(eof bool) {
	b.eof = eof
}

func (b *IoBuffer) copy(expand int) {
	var newBuf []byte
	var bufp *[]byte
	if expand > 0 {
		bufp = b.makeSlice(2*cap(b.buf) + expand)
		newBuf = *bufp
		copy(newBuf, b.buf[b.off:])
		PutBytes(b.b)
		b.b = bufp
	} else if b.off == 0 {
		// 大部分这情况，避免copy无效指令
		newBuf = b.buf
	} else {
		newBuf = b.buf
		copy(newBuf, b.buf[b.off:])
	}
	b.buf = newBuf[:len(b.buf)-b.off]
	b.off = 0
}

func NewIoBuffer(capacity int) *IoBuffer {
	buffer := &IoBuffer{
		offMark: ResetOffMark,
		count:   1,
	}
	if capacity <= 0 {
		capacity = defaultSize
	}

	buffer.b = GetBytes(capacity)
	buffer.buf = (*buffer.b)[:0]
	return buffer
}

func NewIoBufferString(s string) *IoBuffer {
	if s == "" {
		return NewIoBuffer(0)
	}
	return &IoBuffer{
		buf:     []byte(s),
		offMark: ResetOffMark,
		count:   1,
	}
}

func NewIoBufferBytes(bytes []byte) *IoBuffer {
	if bytes == nil {
		return NewIoBuffer(0)
	}
	return &IoBuffer{
		buf:     bytes,
		offMark: ResetOffMark,
		count:   1,
	}
}

func NewIoBufferEOF() *IoBuffer {
	buf := NewIoBuffer(0)
	buf.SetEOF(true)
	return buf
}
