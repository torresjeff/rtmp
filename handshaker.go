package rtmp

import "io"

type Handshaker interface {
	Handshake(reader io.Reader, writer WriteFlusher) error
}
