package rtmp

import (
	"bufio"
	"github.com/pkg/errors"
	"io"
	"testing"
)

const defaultChunkSize = 128

type handshakerMock struct {
	err error
}

var errDuringHandshake = errors.New("error during handshake")

func (h *handshakerMock) Handshake(io.Reader, WriteFlusher) error {
	return h.err
}

func TestNewMessageStream(t *testing.T) {
	reader, _ := NewReader(bufio.NewReader(nil))
	writer, _ := NewWriter(bufio.NewWriter(nil))

	messageStream := NewMessageStream(reader, writer, &handshakerMock{})

	if messageStream.writeChunkSize != defaultChunkSize {
		t.Errorf("expected writeChunkSize to be %v, but got %v", defaultChunkSize, messageStream.writeChunkSize)
	}
	if messageStream.readChunkSize != defaultChunkSize {
		t.Errorf("expected readChunkSize to be %v, but got %v", defaultChunkSize, messageStream.readChunkSize)
	}
	if messageStream.stage != waitingForHandshake {
		t.Errorf("expected stage to be %v, but got %v", waitingForHandshake, messageStream.stage)
	}
}

func TestMessageStream_Initialize(t *testing.T) {
	reader, _ := NewReader(bufio.NewReader(nil))
	writer, _ := NewWriter(bufio.NewWriter(nil))

	initializeTests := []struct {
		name       string
		handshaker Handshaker
		stage      Stage
		out        error
	}{
		{"handshakeSuccessful", &handshakerMock{nil}, handshakeCompleted, nil},
		{"handshakeReturnsError", &handshakerMock{errDuringHandshake}, waitingForHandshake, errDuringHandshake},
	}

	for _, tt := range initializeTests {
		t.Run(tt.name, func(t *testing.T) {
			messageStream := NewMessageStream(reader, writer, tt.handshaker)
			err := messageStream.Initialize()
			if err != tt.out {
				t.Errorf("got %v, want %v", tt.out, err)
			}
			if messageStream.stage != tt.stage {
				t.Errorf("expected stage to be %v, but got %v", handshakeCompleted, messageStream.stage)
			}
		})
	}
}
