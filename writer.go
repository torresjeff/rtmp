package rtmp

import (
	"bufio"
	"io"
)

type Writer struct {
	WriteFlusher

	writer *bufio.Writer
}

type WriteFlusher interface {
	io.Writer
	Flusher
}

type Flusher interface {
	Flush() error
}

func NewWriter(writer *bufio.Writer) (*Writer, error) {
	if writer == nil {
		return nil, ErrNilWriter
	}
	return &Writer{writer: writer}, nil
}

// Write writes the contents of p into the underlying bufio.Writer.
// It returns the number of bytes written.
// If n < len(p), it also returns an error explaining
// why the write is short.
func (w *Writer) Write(p []byte) (n int, err error) {
	return w.writer.Write(p)
}

// Flush writes any buffered data in the underlying bufio.Writer.
func (w *Writer) Flush() error {
	return w.writer.Flush()
}
