package rtmp

import (
	"encoding/binary"
	"errors"
	"github.com/torresjeff/rtmp/internal/binary24"
	"go.uber.org/zap"
)

type Stage uint8

const DefaultChunkSize = 128
const protocolStreamID = 2

const (
	waitingForHandshake Stage = iota
	handshakeCompleted
)

const (
	// Timestamp is in indices [0, 3) (half-open range)
	timestampIndexStart = 0
	timestampLength     = 3

	messageLengthIndexStart = 3
	messageLengthLength     = 3

	messageTypeIDStart = 6

	messageStreamIDStart  = 7
	messageStreamIDLength = 4

	extendedTimestampLength = 4
)

var ErrNextMessageWithoutHandshake = errors.New("NextMessage() was called before completing handshake")
var ErrChunkType1WithoutPreviousChunkType0 = errors.New("received chunk type 1 without previous chunk type 0 in the same chunk stream ID")

type MessageState struct {
	message         *Message
	bytesRead       uint32
	lastChunkHeader *ChunkHeader
}

type MessageStream struct {
	logger         *zap.SugaredLogger
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

func NewMessageStream(logger *zap.SugaredLogger, reader ReadByteReaderCounter, writer WriteFlusher, handshaker Handshaker) *MessageStream {
	return &MessageStream{
		logger:         logger,
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

	chunkHeader, err := ms.parseChunkHeader(chunkType, chunkStreamID)
	if err != nil {
		return nil, err
	}

	ms.logger.Debugf("received new chunk header, %+v", chunkHeader)

	if chunkStreamID == protocolStreamID {
		err = ms.handleProtocolMessage(chunkType)
		if err != nil {
			return nil, err
		}
	} else {
		ms.logger.Info("received non protocol control message")
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

func (ms *MessageStream) handleProtocolMessage(chunkType ChunkType) error {

	return nil
}

func (ms *MessageStream) parseChunkHeader(chunkType ChunkType, chunkStreamID uint32) (*ChunkHeader, error) {
	var timestamp uint32
	var timestampDelta uint32
	var messageLength uint32
	var messageTypeID MessageType
	var messageStreamID uint32
	hasExtendedTimestamp := false
	switch chunkType {
	case ChunkType0:
		messageHeader := make([]byte, chunkType0MessageHeaderLength)
		_, err := ms.reader.Read(messageHeader)
		if err != nil {
			return nil, err
		}

		timestamp = binary24.BigEndian.Uint24(messageHeader[timestampIndexStart:timestampLength])
		if timestamp == 0xFFFFFF {
			hasExtendedTimestamp = true
		}
		messageLength = binary24.BigEndian.Uint24(messageHeader[messageLengthIndexStart : messageLengthIndexStart+messageLengthLength])
		messageTypeID = MessageType(messageHeader[messageTypeIDStart])
		messageStreamID = binary.LittleEndian.Uint32(messageHeader[messageStreamIDStart : messageStreamIDStart+messageStreamIDLength])

		if hasExtendedTimestamp {
			extendedTimestampBytes := make([]byte, extendedTimestampLength)
			timestamp = binary.BigEndian.Uint32(extendedTimestampBytes)
		}
	case ChunkType1:
		previousMessage, exists := ms.messageCache[chunkStreamID]
		if !exists {
			return nil, ErrChunkType1WithoutPreviousChunkType0
		}
		messageHeader := make([]byte, chunkType1MessageHeaderLength)
		_, err := ms.reader.Read(messageHeader)
		if err != nil {
			return nil, err
		}

		timestampDelta = binary24.BigEndian.Uint24(messageHeader[timestampIndexStart:timestampLength])
		if timestampDelta == 0xFFFFFF {
			hasExtendedTimestamp = true
		} else {
			timestamp = previousMessage.lastChunkHeader.timestamp + timestampDelta
		}
		messageLength = binary24.BigEndian.Uint24(messageHeader[messageLengthIndexStart : messageLengthIndexStart+messageLengthLength])
		messageTypeID = MessageType(messageHeader[messageTypeIDStart])
		messageStreamID = previousMessage.lastChunkHeader.messageStreamId

		if hasExtendedTimestamp {
			extendedTimestampBytes := make([]byte, extendedTimestampLength)
			timestampDelta = binary.BigEndian.Uint32(extendedTimestampBytes)
			timestamp = previousMessage.lastChunkHeader.timestamp + timestampDelta
		}
	case ChunkType2:
	case ChunkType3:
	}

	return &ChunkHeader{
		chunkType:       chunkType,
		chunkStreamID:   chunkStreamID,
		timestamp:       timestamp,
		timestampDelta:  timestampDelta,
		messageLength:   messageLength,
		messageType:     messageTypeID,
		messageStreamId: messageStreamID,
	}, nil
}
