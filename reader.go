package rtmp

import (
	"bufio"
	"io"
)

type Reader struct {
	io.Reader

	reader *bufio.Reader
	n      uint64
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

func (r *Reader) getNumberOfReadBytes() uint64 {
	return r.n
}
