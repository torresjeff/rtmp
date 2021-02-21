package rtmp

import (
	"bufio"
	"io"
)

type Writer struct {
	io.Writer

	writer *bufio.Writer
}

// Write writes to the underlying bufio.Writer and calls its Flush method at the end.
// If an error occurs at the Write stage, the number of bytes written and the Write error is returned.
// If an error occurs at the Flush stage, the number of bytes written in the Write stage and the error that happened when flushing is returned.
func (w *Writer) Write(p []byte) (n int, err error) {
	n, err = w.writer.Write(p)
	if err != nil {
		return n, err
	}
	err = w.writer.Flush()
	return n, err
}
