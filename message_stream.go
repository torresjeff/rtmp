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
const max24BitTimestamp = 0xFFFFFF

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

	messageTypeIDIndexStart = 6

	messageStreamIDIndexStart = 7
	messageStreamIDLength     = 4

	extendedTimestampLength = 4
)

var ErrNextMessageWithoutHandshake = errors.New("NextMessage() was called before completing handshake")
var ErrNoPreviousChunkExists = errors.New("received chunk type that depends on a previous chunk, but no previous chunk was found")

type MessageState struct {
	message         *Message
	bytesLeft       uint32
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
	var messageType MessageType
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
		if timestamp == max24BitTimestamp {
			hasExtendedTimestamp = true
		}
		messageLength = binary24.BigEndian.Uint24(messageHeader[messageLengthIndexStart : messageLengthIndexStart+messageLengthLength])
		messageType = MessageType(messageHeader[messageTypeIDIndexStart])
		messageStreamID = binary.LittleEndian.Uint32(messageHeader[messageStreamIDIndexStart : messageStreamIDIndexStart+messageStreamIDLength])

		if hasExtendedTimestamp {
			extendedTimestampBytes := make([]byte, extendedTimestampLength)
			timestamp = binary.BigEndian.Uint32(extendedTimestampBytes)
		}
	case ChunkType1:
		previousMessage, exists := ms.messageCache[chunkStreamID]
		if !exists {
			return nil, ErrNoPreviousChunkExists
		}
		messageHeader := make([]byte, chunkType1MessageHeaderLength)
		_, err := ms.reader.Read(messageHeader)
		if err != nil {
			return nil, err
		}

		timestampDelta = binary24.BigEndian.Uint24(messageHeader[timestampIndexStart:timestampLength])
		if timestampDelta == max24BitTimestamp {
			hasExtendedTimestamp = true
		} else {
			timestamp = previousMessage.lastChunkHeader.timestamp + timestampDelta
		}
		messageLength = binary24.BigEndian.Uint24(messageHeader[messageLengthIndexStart : messageLengthIndexStart+messageLengthLength])
		messageType = MessageType(messageHeader[messageTypeIDIndexStart])
		messageStreamID = previousMessage.lastChunkHeader.messageStreamID

		if hasExtendedTimestamp {
			extendedTimestampBytes := make([]byte, extendedTimestampLength)
			timestampDelta = binary.BigEndian.Uint32(extendedTimestampBytes)
			timestamp = previousMessage.lastChunkHeader.timestamp + timestampDelta
		}
	case ChunkType2:
		previousMessage, exists := ms.messageCache[chunkStreamID]
		if !exists {
			return nil, ErrNoPreviousChunkExists
		}
		messageHeader := make([]byte, chunkType2MessageHeaderLength)
		_, err := ms.reader.Read(messageHeader)
		if err != nil {
			return nil, err
		}

		timestampDelta = binary24.BigEndian.Uint24(messageHeader[timestampIndexStart:timestampLength])
		if timestampDelta == max24BitTimestamp {
			hasExtendedTimestamp = true
		} else {
			timestamp = previousMessage.lastChunkHeader.timestamp + timestampDelta
		}
		messageLength = previousMessage.lastChunkHeader.messageLength
		messageType = previousMessage.lastChunkHeader.messageType
		messageStreamID = previousMessage.lastChunkHeader.messageStreamID

		if hasExtendedTimestamp {
			extendedTimestampBytes := make([]byte, extendedTimestampLength)
			timestampDelta = binary.BigEndian.Uint32(extendedTimestampBytes)
			timestamp = previousMessage.lastChunkHeader.timestamp + timestampDelta
		}
	case ChunkType3:
		previousMessage, exists := ms.messageCache[chunkStreamID]
		if !exists {
			return nil, ErrNoPreviousChunkExists
		}

		newMessage := false
		// Chunk type 3 can be used in two different ways:
		// 1) to specify the continuation of a message.
		// 2) to specify the beginning of a new message whose header can be derived from the existing state data.
		if previousMessage.bytesLeft > 0 {
			// This chunk type 3 is the continuation of a message, so preserve the same timestamp delta as the previous chunk
			timestampDelta = previousMessage.lastChunkHeader.timestamp
		} else {
			newMessage = true
			// if a chunk type 3 follows a chunk type 0 (and is a new message), then the delta is the absolute timestamp of the type 0 chunk
			if previousMessage.lastChunkHeader.chunkType == ChunkType0 {
				timestampDelta = previousMessage.lastChunkHeader.timestamp
			} else {
				timestampDelta = previousMessage.lastChunkHeader.timestampDelta
			}
		}

		if timestampDelta >= max24BitTimestamp {
			hasExtendedTimestamp = true
		}

		messageLength = previousMessage.lastChunkHeader.messageLength
		messageType = previousMessage.lastChunkHeader.messageType
		messageStreamID = previousMessage.lastChunkHeader.messageStreamID

		if hasExtendedTimestamp {
			extendedTimestampBytes := make([]byte, extendedTimestampLength)
			timestampDelta = binary.BigEndian.Uint32(extendedTimestampBytes)
		}

		if newMessage {
			timestamp = previousMessage.lastChunkHeader.timestamp + timestampDelta
		} else {
			timestamp = previousMessage.lastChunkHeader.timestamp
		}
	}

	chunkHeader := &ChunkHeader{
		chunkType:       chunkType,
		chunkStreamID:   chunkStreamID,
		timestamp:       timestamp,
		timestampDelta:  timestampDelta,
		messageLength:   messageLength,
		messageType:     messageType,
		messageStreamID: messageStreamID,
	}

	return chunkHeader, nil
}
