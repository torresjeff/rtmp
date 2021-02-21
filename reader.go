package rtmp

import (
	"bufio"
	"io"
)

type Reader struct {
	io.Reader

	reader *bufio.Reader
}

// Read reads exactly len(p) bytes from the underlying bufio.Reader into p.
// It returns the number of bytes copied and an error if fewer bytes were read.
func (r *Reader) Read(p []byte) (n int, err error) {
	return io.ReadFull(r.reader, p)
}

// ReadByte reads and returns a single byte from the underlying bufio.Reader.
// If no byte is available, returns an error.
func (r *Reader) ReadByte() (byte, error) {
	return r.reader.ReadByte()
}
