package rtmp

import (
	"bufio"
	"io"
)

type Reader struct {
	ReadByteReaderCounter

	reader *bufio.Reader
	n      uint64
}

type ByteCounter interface {
	ReadBytes() uint64
}

type ByteReader interface {
	ReadByte() (byte, error)
}

// ReadByteReaderCounter is the interface that groups Reader, ByteReader, and ByteCounter interfaces.
type ReadByteReaderCounter interface {
	io.Reader
	ByteCounter
	ByteReader
}

func NewReader(reader *bufio.Reader) (*Reader, error) {
	if reader == nil {
		return nil, ErrNilReader
	}
	return &Reader{reader: reader}, nil
}

// Read reads exactly len(p) bytes from the underlying bufio.Reader into p.
// It returns the number of bytes copied and an error if fewer bytes were read.
// The error is EOF only if no bytes were read.
// If an EOF happens after reading some but not all the bytes,
// Read returns ErrUnexpectedEOF.
// On return, n == len(buf) if and only if err == nil.
func (r *Reader) Read(p []byte) (n int, err error) {
	n, err = io.ReadFull(r.reader, p)
	r.n += uint64(n)
	return n, err
}

// ReadByte reads and returns a single byte from the underlying bufio.Reader.
// If no byte is available, returns an error.
func (r *Reader) ReadByte() (byte, error) {
	b, err := r.reader.ReadByte()
	if err == nil {
		r.n++
	}

	return b, err
}

// Returns the number of bytes read so far from the underlying bufio.Reader since the instantiation of the Reader.
func (r *Reader) ReadBytes() uint64 {
	return r.n
}
