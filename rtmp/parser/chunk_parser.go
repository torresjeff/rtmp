package parser

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"github.com/pkg/errors"
	"github.com/torresjeff/rtmp-server/config"
	"io"
)

// Chunk types
const (
	ChunkType0 = iota
	ChunkType1
	ChunkType2
	ChunkType3
)

// Control message types
const (
	// Control messages MUST have message stream ID 0 and be sent in chunk stream ID 2
	SetChunkSize = 1
	AbortMessage = 2
	Ack = 3
	WindowAckSize = 5
	SetPeerBandwidth = 6

	UserControlMessage = 4
)

// Types of messages and commands
const (
	CommandMessageAMF0 = 20
	CommandMessageAMF3 = 17

	DataMessageAMF0 = 18
	DataMessageAMF3 = 15

	SharedObjectMessageAMF0 = 19
	SharedObjectMessageAMF3 = 16

	AudioMessage = 8
	VideoMessage = 9
	AggregateMessage = 22

)

const DefaultMaximumChunkSize = 128


type ChunkParser struct {
	reader          *bufio.Reader
	prevChunkHeader *ChunkHeader
	inChunkSize     uint32
	windowAckSize   uint32
	bytesReceived   uint32
	outBandwidth    uint32
	limit           uint8
}

type Chunk struct {
	Header *ChunkHeader
	Body *ChunkData
}

type ChunkHeader struct {
	BasicHeader *ChunkBasicHeader
	MessageHeader *ChunkMessageHeader
	ExtendedTimestamp uint32
}

type ChunkData struct {
	payload []byte
}

type ChunkBasicHeader struct {
	// Chunk type
	FMT uint8
	ChunkStreamID uint32
}

type ChunkMessageHeader struct {
	// Absolute timestamp of the message (used for Type 0 chunks only)
	Timestamp uint32
	MessageLength uint32
	MessageTypeID uint8
	MessageStreamID uint32
	// Used for Type 1 or Type 2 chunks (see: RTMP spec, pg. 15)
	TimestampDelta uint32
}

func NewChunkParser(reader *bufio.Reader) *ChunkParser {
	return &ChunkParser{
		reader:      reader,
		inChunkSize: DefaultMaximumChunkSize,
	}
}

func (parser *ChunkParser) ReadChunkHeader() (*ChunkHeader, error) {
	ch := &ChunkHeader{}
	var err error
	if err = parser.readBasicHeader(ch); err != nil {
		return nil, err
	}
	if err = parser.readMessageHeader(ch); err != nil {
		return nil, err
	}

	// Check if this chunk has an extended timestamp, and if it does then read it. A Timestamp or TimestampDelta of 0xFFFFFF indicates an extended timestamp.
	if ch.MessageHeader.Timestamp == 0xFFFFFF || ch.MessageHeader.TimestampDelta == 0xFFFFFF {
		if err = parser.readExtendedTimestamp(ch); err != nil {
			return nil, err
		}
	}

	// TODO: should I really store previous chunk header? or is this only useful for RTMP clients?
	parser.prevChunkHeader = ch
	return ch, nil
}

func (parser *ChunkParser) ReadChunkData(header *ChunkHeader) (*ChunkData, error) {
	switch header.MessageHeader.MessageTypeID {
	case SetChunkSize, AbortMessage, Ack, WindowAckSize, SetPeerBandwidth:
		return parser.handleControlMessage(header)
	case UserControlMessage:
		return parser.handleUserControlMessage(header)
	case CommandMessageAMF0, CommandMessageAMF3:
		//return parser.handleCommandMessage(header.MessageHeader.MessageTypeID)
	}

	return nil, nil
}

func (parser *ChunkParser) readBasicHeader(header *ChunkHeader) error {
	basicHeader := &ChunkBasicHeader{}

	b, err := parser.reader.ReadByte()
	if err != nil {
		return err
	}
	// Extract chunk type (FMT field) by getting the last 2 bits (bit 6 and 7 store fmt)
	basicHeader.FMT = uint8(b) >> 6
	// Get the chunk stream ID (first 6 bits, bits 0-5). 0x3F == 0011 1111 in binary (our bit mask to extract the first 6 bits)
	csid := b & uint8(0x3F)

	if csid == 0 {
		// if csid is 0, that means we're dealing with chunk basic header 2 (uses 2 bytes)
		id, err := parser.reader.ReadByte()
		if err != nil {
			return err
		}
		basicHeader.ChunkStreamID = uint32(id) + 64

	} else if csid == 1 {
		// if csid is 1, that means we're dealing with chunk basic header 3 (uses 3 bytes).
		id := make([]byte, 2)
		_, err := io.ReadAtLeast(parser.reader, id, 2)
		if err != nil {
			return err
		}
		basicHeader.ChunkStreamID = uint32(binary.BigEndian.Uint16(id))+ 64

	} else {
		// if csid is neither 0 or 1, that means we're dealing with chunk basic header 1 (uses 1 byte). This represents the actual chunk stream ID.
		basicHeader.ChunkStreamID = uint32(csid)
	}

	header.BasicHeader = basicHeader
	return nil
}

func (parser *ChunkParser) readMessageHeader(header *ChunkHeader) error {
	mh := &ChunkMessageHeader{}
	switch header.BasicHeader.FMT {
	case ChunkType0:
		if config.Debug {
			fmt.Println("chunk type 0 detected")
		}
		messageHeader := make([]byte, 11)
		// A chunk of type 0 has a message header size of 11 bytes, so read 11 bytes into our messageHeader buffer
		_, err := io.ReadAtLeast(parser.reader, messageHeader, 11)
		if err != nil {
			return err
		}
		// Since the timestamp field is 3 bytes long, to be able to interpret it as a 32-bit uint we have to add 1 byte at the beginning (3 + 1 byte = 4 bytes == 32-bits)
		mh.Timestamp = binary.BigEndian.Uint32(append([]byte{0x00}, messageHeader[:3]...))
		// Same for the MessageLength field (3 bytes long as well, so add 1 to the beginning)
		mh.MessageLength = binary.BigEndian.Uint32(append([]byte{0x00}, messageHeader[3:6]...))
		// Message type ID is only 1 byte, so read the byte directly
		mh.MessageTypeID = uint8(messageHeader[6])
		// Finally, read the message stream id (remaining 4 bytes)
		mh.MessageStreamID = binary.BigEndian.Uint32(messageHeader[7:])

		header.MessageHeader = mh

		return nil
	case ChunkType1:
		if config.Debug {
			fmt.Println("chunk type 1 detected")
		}
		messageHeader := make([]byte, 7)
		// A chunk of type 1 has a message header size of 7 bytes, so read 7 bytes into our messageHeader buffer
		_, err := io.ReadAtLeast(parser.reader, messageHeader, 7)
		if err != nil {
			return err
		}
		// Since the timestamp delta field is 3 bytes long, to be able to interpret it as a 32-bit uint we have to add 1 byte at the beginning (3 + 1 byte = 4 bytes == 32-bits)
		// NOTE: this uses the TimestampDelta field, not the Timestamp field (which is only used for chunk type 0)
		mh.TimestampDelta = binary.BigEndian.Uint32(append([]byte{0x00}, messageHeader[:3]...))
		// Same for the MessageLength field (3 bytes long as well, so add 1 to the beginning)
		mh.MessageLength = binary.BigEndian.Uint32(append([]byte{0x00}, messageHeader[3:6]...))
		// Message type ID is only 1 byte, so read the byte directly
		mh.MessageTypeID = uint8(messageHeader[6])
		// Chunk type 1 message headers don't have a message stream ID. This chunk takes the same message stream ID as the previous chunk.
		mh.MessageStreamID = parser.prevChunkHeader.MessageHeader.MessageStreamID

		header.MessageHeader = mh
		return nil
	case ChunkType2:
		if config.Debug {
			fmt.Println("chunk type 2 detected")
		}
		messageHeader := make([]byte, 3)
		// A chunk of type 1 has a message header size of 3 bytes, so read 3 bytes into our messageHeader buffer
		_, err := io.ReadAtLeast(parser.reader, messageHeader, 3)
		if err != nil {
			return err
		}
		// Since the timestamp delta field is 3 bytes long, to be able to interpret it as a 32-bit uint we have to add 1 byte at the beginning (3 + 1 byte = 4 bytes == 32-bits)
		// NOTE: this uses the TimestampDelta field, not the Timestamp field (which is only used for chunk type 0)
		mh.TimestampDelta = binary.BigEndian.Uint32(append([]byte{0x00}, messageHeader[:3]...))
		// Chunk type 2 message headers don't have a message length. This chunk takes the same message length as the previous chunk.
		mh.MessageLength = parser.prevChunkHeader.MessageHeader.MessageLength
		// Chunk type 2 message headers don't have a message type ID. This chunk takes the same message type ID as the previous chunk.
		// TODO: not sure about this. The spec doesn't say anything about Message Type ID for type 2 chunks
		mh.MessageTypeID = parser.prevChunkHeader.MessageHeader.MessageTypeID
		// Chunk type 2 message headers don't have a message stream ID. This chunk takes the same message stream ID as the previous chunk.
		mh.MessageStreamID = parser.prevChunkHeader.MessageHeader.MessageStreamID

		header.MessageHeader = mh
		return nil
	case ChunkType3:
		if config.Debug {
			fmt.Println("chunk type 3 detected")
		}
		// Chunk type 3 message headers don't have any data. All values are taken from the previous header.
		mh.TimestampDelta = parser.prevChunkHeader.MessageHeader.TimestampDelta
		mh.MessageLength = parser.prevChunkHeader.MessageHeader.MessageLength
		// TODO: not sure about this. The spec doesn't say anything about Message Type ID for type 2 chunks
		mh.MessageTypeID = parser.prevChunkHeader.MessageHeader.MessageTypeID
		mh.MessageStreamID = parser.prevChunkHeader.MessageHeader.MessageStreamID
	}
	return nil
}

func (parser *ChunkParser) readExtendedTimestamp(header *ChunkHeader) error {
	extendedTimestamp := make([]byte, 4)
	_, err := io.ReadAtLeast(parser.reader, extendedTimestamp, 4)
	if err != nil {
		return err
	}
	header.ExtendedTimestamp = binary.BigEndian.Uint32(extendedTimestamp)
	return nil
}

func (parser *ChunkParser) handleControlMessage(header *ChunkHeader) (*ChunkData, error) {
	messageLength := header.MessageHeader.MessageLength
	switch header.MessageHeader.MessageTypeID {
	case SetChunkSize:
		if config.Debug {
			fmt.Println("Received SetChunkSize control message")
		}
		// The payload of a set chunk size message is the new chunk size
		chunkSize := make([]byte, messageLength)
		_, err := io.ReadAtLeast(parser.reader, chunkSize, int(messageLength))
		if err != nil {
			return nil, err
		}
		parser.inChunkSize = binary.BigEndian.Uint32(chunkSize)
		if config.Debug {
			fmt.Println("Set inChunkSize to", parser.inChunkSize)
		}

		return &ChunkData{
			payload: chunkSize,
		}, nil
	case AbortMessage:
		// The payload of an abort message is the chunk stream ID whose current message is to be discarded
		chunkStreamId := make([]byte, messageLength)
		_, err := io.ReadAtLeast(parser.reader, chunkStreamId, int(messageLength))
		if err != nil {
			return nil, err
		}
		return &ChunkData{
			payload: chunkStreamId,
		}, nil
	case Ack:
		// The payload of an ack message is the sequence number (number of bytes received so far)
		sequenceNumber := make([]byte, messageLength)
		_, err := io.ReadAtLeast(parser.reader, sequenceNumber, int(messageLength))
		if err != nil {
			return nil, err
		}
		return &ChunkData{
			payload: sequenceNumber,
		}, nil
	case WindowAckSize:
		// The payload of a window ack size is the window size to use between acknowledgements
		ackWindowSize := make([]byte, messageLength)
		_, err := io.ReadAtLeast(parser.reader, ackWindowSize, int(messageLength))
		if err != nil {
			return nil, err
		}

		// the ack window size is in the first 4 bytes
		parser.windowAckSize = binary.BigEndian.Uint32(ackWindowSize[:4])

		return &ChunkData{
			payload: ackWindowSize,
		}, nil
	case SetPeerBandwidth:
		// The payload of a set peer bandwidth message is the window size and the limit type (hard, soft, dynamic)
		ackWindowSize := make([]byte, messageLength)
		_, err := io.ReadAtLeast(parser.reader, ackWindowSize, int(messageLength))
		if err != nil {
			return nil, err
		}
		// the ack window size is in the first 4 bytes
		//parser.windowAckSize = binary.BigEndian.Uint32(ackWindowSize[:4])
		// for now, ignore the limit type in byte 5
		//limitType := ackWindowSize[4:]

		return &ChunkData{
			payload: ackWindowSize,
		}, nil
	default:
		return nil, errors.New("received unsupported message type ID")
	}
}

func (parser *ChunkParser) handleUserControlMessage(header *ChunkHeader) (*ChunkData, error) {
	messageLength := header.MessageHeader.MessageLength
	// The first 2 bytes of the payload define the event type, the rest of the payload is the event data (the size of event data is variable)
	payload := make([]byte, messageLength)
	_, err := io.ReadAtLeast(parser.reader, payload, int(messageLength))
	if err != nil {
		return nil, err
	}
	return &ChunkData{
		payload: payload,
	}, nil
}