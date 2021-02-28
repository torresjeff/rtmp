package rtmp

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type Stage uint8

const DefaultChunkSize = 128

const (
	waitingForHandshake Stage = iota
	handshakeCompleted
)

var ErrNextMessageWithoutHandshake = errors.New("NextMessage() was called before completing handshake")

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
	messageCache map[uint32]MessageState
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
		messageCache:   make(map[uint32]MessageState),
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
	if ms.stage == waitingForHandshake {
		return nil, ErrNextMessageWithoutHandshake
	}
	basicHeader, err := ms.reader.ReadByte()
	if err != nil {
		return nil, err
	}

	chunkType := ChunkType(basicHeader >> 6)
	chunkStreamID, err := ms.getChunkStreamID(basicHeader)
	if err != nil {
		return nil, err
	}
	fmt.Println("chunkType: ", chunkType, ", chunkStreamID: ", chunkStreamID)

	switch chunkType {
	case ChunkType0:
	case ChunkType1:
	case ChunkType2:

	}
	return nil, nil
}

func (ms *MessageStream) getChunkStreamID(basicHeader uint8) (uint32, error) {
	chunkStreamID := uint32(basicHeader & 0x3F)
	// The protocol supports up to 65597 streams with IDs 3-65599. The IDs 0, 1, and 2 are reserved.
	// Value 0 indicates the 2 byte form and an ID in the range of 64-319 (the second byte + 64).
	// Value 1 indicates the 3 byte form and an ID in the range of 64-65599 ((the third byte)*256 + the second byte + 64).
	// Values in the range of 3-63 represent the complete stream ID. Chunk Stream ID with value 2 is reserved for low-level protocol control messages and commands.
	if chunkStreamID == 0 {
		csID, err := ms.reader.ReadByte()
		if err != nil {
			return 0, err
		}
		chunkStreamID = uint32(csID) + 64
	} else if chunkStreamID == 1 {
		csIDBytes := make([]byte, 2)
		_, err := ms.reader.Read(csIDBytes)
		if err != nil {
			return 0, err
		}
		chunkStreamID = uint32(binary.BigEndian.Uint16(csIDBytes)) + 64
	}
	return chunkStreamID, nil
}
