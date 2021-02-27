package rtmp

type Stage uint8

const DefaultChunkSize = 128

const (
	waitingForHandshake Stage = iota
	handshakeCompleted
)

type MessageState struct {
	message         *Message
	bytesRead       uint32
	lastChunkHeader *ChunkHeader
}

type MessageStream struct {
	handshaker     Handshaker
	reader         ReadByteReaderCounter
	writer         WriteFlusher
	readChunkSize  uint32
	writeChunkSize uint32
	// messageCache maps the chunk stream ID to information about the previously received message on that same chunk stream ID.
	// We need this to form a complete message in case it's divided up into multiple chunks (since they can be interleaved).
	messageCache map[uint64]MessageState
	// stage represents the current state of the message stream. Initially set to waitingForHandshake.
	// An attempt to call NextMessage() or SendMessage() in the message stream will result in an error if the stage is set to waitingForHandshake.
	stage Stage
}

func NewMessageStream(reader ReadByteReaderCounter, writer WriteFlusher, handshaker Handshaker) *MessageStream {
	return &MessageStream{
		handshaker:     handshaker,
		reader:         reader,
		writer:         writer,
		readChunkSize:  DefaultChunkSize,
		writeChunkSize: DefaultChunkSize,
		messageCache:   make(map[uint64]MessageState),
		stage:          waitingForHandshake,
	}
}

// Initialize performs the handshake and changes the internal state of the MessageStream to handshakeCompleted
func (ms *MessageStream) Initialize() error {
	err := ms.handshaker.Handshake(ms.reader, ms.writer)
	if err != nil {
		return err
	}
	ms.stage = handshakeCompleted
	return nil
}

func (ms *MessageStream) NextMessage() (*Message, error) {
	// TODO: implement
	return nil, nil
}
