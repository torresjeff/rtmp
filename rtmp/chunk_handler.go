package rtmp

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

const (
	LimitHard uint8 = 0
	LimitSoft uint8 = 1
	LimitDynamic uint8 = 2
	// Not part of the spec, it's for our internal use when a LimitDynamic message comes in
	LimitNotSet uint8 = 3
)

// Chunk handler is in charge of reading chunk headers and data. It will assemble a message from multiple chunks if it has to.
type ChunkHandler struct {
	socket          *bufio.ReadWriter
	prevChunkHeader *ChunkHeader
	inChunkSize     uint32
	outChunkSize     uint32
	windowAckSize   uint32
	bytesReceived   uint32
	outBandwidth    uint32
	limit           uint8

	// False if no Acknowledgement message has been sent yet
	ackSent bool
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

// TODO: use the callback
func NewChunkHandler(reader *bufio.ReadWriter) *ChunkHandler {
	return &ChunkHandler{
		socket:             reader,
		inChunkSize:        DefaultMaximumChunkSize,
		outChunkSize: DefaultMaximumChunkSize,
		ackSent:            false,
	}
}

func (chunkHandler *ChunkHandler) ReadChunkHeader() (*ChunkHeader, error) {
	ch := &ChunkHeader{}
	var err error
	if err = chunkHandler.readBasicHeader(ch); err != nil {
		return nil, err
	}
	if err = chunkHandler.readMessageHeader(ch); err != nil {
		return nil, err
	}

	// Check if this chunk has an extended timestamp, and if it does then read it. A Timestamp or TimestampDelta of 0xFFFFFF indicate an extended timestamp.
	if ch.MessageHeader.Timestamp == 0xFFFFFF || ch.MessageHeader.TimestampDelta == 0xFFFFFF {
		if err = chunkHandler.readExtendedTimestamp(ch); err != nil {
			return nil, err
		}
	}

	// TODO: should I really store previous chunk header? or is this only useful for RTMP clients?
	chunkHandler.prevChunkHeader = ch
	return ch, nil
}

// assembleMessage is called when the length of a message is greater than the currently set chunkSize.
// It returns the final payload of the message assembled from multiple chunks.
func (chunkHandler *ChunkHandler) assembleMessage(messageLength uint32) ([]byte, error) {
	payload := make([]byte, messageLength)

	// Read the initial chunk data that was sent with the first chunk header
	_, err := io.ReadFull(chunkHandler.socket, payload[:chunkHandler.inChunkSize])
	if err != nil {
		return nil, err
	}
	// Update the number of bytes read to the inChunkSize since we already read at least inChunkSize bytes
	bytesRead := chunkHandler.inChunkSize
	// While there are still more bytes to read
	for bytesRead < messageLength {
		// Read the next chunks (header + data) until we complete our message
		_, err := chunkHandler.ReadChunkHeader()
		if err != nil {
			return nil, errors.New("error reading chunk while attempting to assemble a multi-chunk message" + err.Error())
		}
		// If this chunk is still not the end of the message, then read the whole chunk
		if bytesRead + chunkHandler.inChunkSize < messageLength {
			_, err := io.ReadFull(chunkHandler.socket, payload[bytesRead:bytesRead + chunkHandler.inChunkSize])
			if err != nil {
				return nil, err
			}
			bytesRead += chunkHandler.inChunkSize
		} else {
			// If this is the last chunk of the message, just read the remaining bytes
			remainingBytes := messageLength - bytesRead
			_, err := io.ReadFull(chunkHandler.socket, payload[bytesRead:bytesRead + remainingBytes])
			if err != nil {
				return nil, err
			}
			bytesRead += remainingBytes
		}
	}
	return payload, nil
}

func (chunkHandler *ChunkHandler) ReadChunkData(header *ChunkHeader) ([]byte, error) {
	messageLength := header.MessageHeader.MessageLength
	var payload []byte
	// Check if the length of the message is greater than the chunk size (default chunk size is 128 if no Set Chunk Size message has been received).
	// If it is, we have to assemble the complete message from various chunks.
	if messageLength > chunkHandler.inChunkSize {
		messagePayload, err := chunkHandler.assembleMessage(messageLength)
		if err != nil {
			return nil, err
		}
		payload = messagePayload
	} else {
		payload = make([]byte, messageLength)
		_, err := io.ReadAtLeast(chunkHandler.socket, payload, int(messageLength))
		if err != nil {
			return nil, err
		}
	}

	return payload, nil
}

func (chunkHandler *ChunkHandler) readBasicHeader(header *ChunkHeader) error {
	basicHeader := &ChunkBasicHeader{}

	b, err := chunkHandler.socket.ReadByte()
	if err != nil {
		return err
	}
	// Extract chunk type (FMT field) by getting the 2 highest bits (bit 6 and 7 store fmt)
	basicHeader.FMT = uint8(b) >> 6
	// Get the chunk stream ID (first 6 bits, bits 0-5). 0x3F == 0011 1111 in binary (our bit mask to extract the lowest 6 bits)
	csid := b & uint8(0x3F)

	if csid == 0 {
		// if csid is 0, that means we're dealing with chunk basic header 2 (uses 2 bytes)
		id, err := chunkHandler.socket.ReadByte()
		if err != nil {
			return err
		}
		basicHeader.ChunkStreamID = uint32(id) + 64
	} else if csid == 1 {
		// if csid is 1, that means we're dealing with chunk basic header 3 (uses 3 bytes).
		id := make([]byte, 2)
		_, err := io.ReadAtLeast(chunkHandler.socket, id, 2)
		if err != nil {
			return err
		}
		basicHeader.ChunkStreamID = uint32(binary.BigEndian.Uint16(id))+ 64
		chunkHandler.bytesReceived += 2
	} else {
		// if csid is neither 0 or 1, that means we're dealing with chunk basic header 1 (uses 1 byte).
		basicHeader.ChunkStreamID = uint32(csid)
	}

	header.BasicHeader = basicHeader
	return nil
}

func (chunkHandler *ChunkHandler) readMessageHeader(header *ChunkHeader) error {
	mh := &ChunkMessageHeader{}
	switch header.BasicHeader.FMT {
	case ChunkType0:
		messageHeader := make([]byte, 11)
		// A chunk of type 0 has a message header size of 11 bytes, so read 11 bytes into our messageHeader buffer
		_, err := io.ReadAtLeast(chunkHandler.socket, messageHeader, 11)
		if err != nil {
			return err
		}
		// Since the timestamp field is 3 bytes long, to be able to interpret it as a 32-bit uint we have to add 1 byte at the beginning (3 + 1 byte = 4 bytes == 32-bits)
		mh.Timestamp = binary.BigEndian.Uint32(append([]byte{0x00}, messageHeader[:3]...))
		// Same for the MessageLength field (3 bytes long as well, so add 1 to the beginning)
		mh.MessageLength = binary.BigEndian.Uint32(append([]byte{0x00}, messageHeader[3:6]...))
		// Message type ID is only 1 byte, so read the byte directly
		mh.MessageTypeID = uint8(messageHeader[6])
		// Finally, read the message stream sessionID (remaining 4 bytes)
		// NOTE: message stream ID is stored in little endian format
		mh.MessageStreamID = binary.LittleEndian.Uint32(messageHeader[7:])

		header.MessageHeader = mh
		return nil
	case ChunkType1:
		messageHeader := make([]byte, 7)
		// A chunk of type 1 has a message header size of 7 bytes, so read 7 bytes into our messageHeader buffer
		_, err := io.ReadAtLeast(chunkHandler.socket, messageHeader, 7)
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
		mh.MessageStreamID = chunkHandler.prevChunkHeader.MessageHeader.MessageStreamID

		header.MessageHeader = mh
		return nil
	case ChunkType2:
		messageHeader := make([]byte, 3)
		// A chunk of type 1 has a message header size of 3 bytes, so read 3 bytes into our messageHeader buffer
		_, err := io.ReadAtLeast(chunkHandler.socket, messageHeader, 3)
		if err != nil {
			return err
		}
		// Since the timestamp delta field is 3 bytes long, to be able to interpret it as a 32-bit uint we have to add 1 byte at the beginning (3 + 1 byte = 4 bytes == 32-bits)
		// NOTE: this uses the TimestampDelta field, not the Timestamp field (which is only used for chunk type 0)
		mh.TimestampDelta = binary.BigEndian.Uint32(append([]byte{0x00}, messageHeader[:3]...))
		// Chunk type 2 message headers don't have a message length. This chunk takes the same message length as the previous chunk.
		mh.MessageLength = chunkHandler.prevChunkHeader.MessageHeader.MessageLength
		// Chunk type 2 message headers don't have a message stream ID. This chunk takes the same message stream ID as the previous chunk.
		mh.MessageStreamID = chunkHandler.prevChunkHeader.MessageHeader.MessageStreamID
		// Chunk type 2 message headers don't have a message type ID. This chunk takes the same message type ID as the previous chunk.
		mh.MessageTypeID = chunkHandler.prevChunkHeader.MessageHeader.MessageTypeID

		header.MessageHeader = mh
		return nil
	case ChunkType3:
		// Chunk type 3 message headers don't have any data. All values are taken from the previous header.

		// As per the spec: If a Type 3 chunk follows a Type 0 chunk, then the timestamp delta for this Type 3 chunk is the same as the timestamp of the Type 0 chunk.
		if chunkHandler.prevChunkHeader.BasicHeader.FMT == ChunkType0 {
			mh.TimestampDelta = chunkHandler.prevChunkHeader.MessageHeader.Timestamp
		} else {
			mh.TimestampDelta = chunkHandler.prevChunkHeader.MessageHeader.TimestampDelta
		}
		mh.MessageLength = chunkHandler.prevChunkHeader.MessageHeader.MessageLength
		mh.MessageTypeID = chunkHandler.prevChunkHeader.MessageHeader.MessageTypeID
		mh.MessageStreamID = chunkHandler.prevChunkHeader.MessageHeader.MessageStreamID
		header.MessageHeader = mh
		return nil
	}
	return nil
}

func (chunkHandler *ChunkHandler) readExtendedTimestamp(header *ChunkHeader) error {
	extendedTimestamp := make([]byte, 4)
	_, err := io.ReadAtLeast(chunkHandler.socket, extendedTimestamp, 4)
	if err != nil {
		return err
	}
	header.ExtendedTimestamp = binary.BigEndian.Uint32(extendedTimestamp)
	return nil
}

func (chunkHandler *ChunkHandler) updateBytesReceived(i uint32) {
	chunkHandler.bytesReceived += i
	// TODO: implement send ack
	if chunkHandler.bytesReceived >= chunkHandler.windowAckSize {
		chunkHandler.sendAck()
	}
}

// TODO: handle errors for all of these functions
func (chunkHandler *ChunkHandler) sendWindowAckSize(size uint32) {
	message := generateWindowAckSizeMessage(size)
	// TODO: wrap the socket in a more user friendly struct that uses Write and Flush in one method
	chunkHandler.socket.Write(message)
	chunkHandler.socket.Flush()
}

func (chunkHandler *ChunkHandler) sendSetPeerBandWidth(size uint32, limit uint8) {
	message := generateSetPeerBandwidthMessage(size, limit)
	chunkHandler.socket.Write(message)
	chunkHandler.socket.Flush()
}

func (chunkHandler *ChunkHandler) sendBeginStream(streamID uint32) {
	message := generateStreamBeginMessage(streamID)
	chunkHandler.socket.Write(message)
	chunkHandler.socket.Flush()
}

func (chunkHandler *ChunkHandler) sendSetChunkSize(size uint32) {
	message := generateSetChunkSizeMessage(size)
	chunkHandler.socket.Write(message)
	chunkHandler.socket.Flush()
	chunkHandler.outChunkSize = size
}

func (chunkHandler *ChunkHandler) sendConnectSuccess(csID uint32) {
	message := generateConnectResponseSuccess(csID)
	chunkHandler.socket.Write(message)
	chunkHandler.socket.Flush()
}

func (chunkHandler *ChunkHandler) sendAck() {
	message := generateAckMessage(chunkHandler.bytesReceived)
	chunkHandler.socket.Write(message)
	chunkHandler.socket.Flush()
	// Reset the number of bytes received
	chunkHandler.bytesReceived = 0
	chunkHandler.ackSent = true
}

func (chunkHandler *ChunkHandler) SetChunkSize(size uint32) {
	if config.Debug {
		fmt.Println("Set chunk size to", size)
	}
	chunkHandler.inChunkSize = size
}

// Sets the window acknowledgement size to the new size
func (chunkHandler *ChunkHandler) SetWindowAckSize(size uint32) {
	if config.Debug {
		fmt.Println("Set window ack size to", size)
	}
	// If no acknowledgement has been sent since the beginning of the session, send it
	if !chunkHandler.ackSent {
		chunkHandler.sendAck()
	}
	chunkHandler.windowAckSize = size
}

func (chunkHandler *ChunkHandler) SetBandwidth(size uint32, limitType uint8) {
	// For now, ignore the limitType. Treat it as a hard limit (always set the window size)
	// TODO: what is the purpose of set bandwidth?
	//chunkHandler.SetWindowAckSize(size)
}

func (chunkHandler *ChunkHandler) send(header []byte, payload []byte) error {
	_, err := chunkHandler.socket.Write(header)
	if err != nil {
		return err
	}

	// Determine if we have to chunk our payload
	if len(payload) > int(chunkHandler.outChunkSize) {
		payloadLength := len(payload)
		// TODO: whenever we fragment a message, is it always with type 3 chunks?
		// take whatever csid came in the original header, and use it for future chunks. Also specify fmt = 3 (chunk header - type 3) for subsequent chunks
		chunk3Header := (3 << 6) | (header[0] & 0x3F)

		chunkSize := int(chunkHandler.outChunkSize)
		bytesWritten := 0 // bytes of the PAYLOAD we've written
		// True if this is the first time we're going to write payload data in a chunk
		firstPayloadChunk := true
		for bytesWritten < payloadLength {
			if !firstPayloadChunk {
				// We've already written payload data, so separate it with a chunk type 3 header
				err = chunkHandler.socket.WriteByte(chunk3Header)
				if err != nil {
					return err
				}
			} else {
				firstPayloadChunk = false
			}
			// if the next chunk is still not the end of the message, write chunk size bytes
			if bytesWritten + chunkSize < payloadLength {
				_, err = chunkHandler.socket.Write(payload[bytesWritten:bytesWritten + chunkSize])
				if err != nil {
					return err
				}
				bytesWritten += chunkSize
			} else {
				// Write remaining data
				remainingBytes := payloadLength - bytesWritten
				_, err = chunkHandler.socket.Write(payload[bytesWritten:bytesWritten + remainingBytes])
				bytesWritten += remainingBytes
			}
		}
	} else {
		// No chunking needed
		_, err := chunkHandler.socket.Write(payload)
		if err != nil {
			return err
		}
	}

	err = chunkHandler.socket.Flush()
	if err != nil {
		return err
	}

	return nil
}

func (chunkHandler *ChunkHandler) sendBytes(bytes []byte) {
	chunkHandler.socket.Write(bytes)
	chunkHandler.socket.Flush()
}