package rtmp

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"github.com/pkg/errors"
	"github.com/torresjeff/rtmp/config"
	"io"
)

var InvalidChunkType error = errors.New("chunk handler: unknown chunk type")

// Chunk types
const (
	ChunkType0 uint8 = 0
	ChunkType1 uint8 = 1
	ChunkType2 uint8 = 2
	ChunkType3 uint8 = 3
)

const (
	// Only Protocol Channel is defined in the spec (csid = 2), the others are defined by me with the idea of being
	// consistent in sending the same type of data through the same chunk stream id
	ProtocolChannel uint8 = 2
	AudioChannel    uint8 = 4
	VideoChannel    uint8 = 7
)

const DefaultMaximumChunkSize = 128

const (
	LimitHard    uint8 = 0
	LimitSoft    uint8 = 1
	LimitDynamic uint8 = 2
	// Not part of the spec, it's for our internal use when a LimitDynamic message comes in
	LimitNotSet uint8 = 3
)

// Chunk handler is in charge of reading chunk headers and data. It will assemble a message from multiple chunks if it has to.
type ChunkHandler struct {
	socketr *bufio.Reader
	socketw *bufio.Writer
	// The key is the chunk stream ID, and the value is the previous header of that chunk stream ID
	prevChunkHeader map[uint32]ChunkHeader
	inChunkSize     uint32
	outChunkSize    uint32
	windowAckSize   uint32
	bytesReceived   uint32
	outBandwidth    uint32
	limit           uint8

	// False if no Acknowledgement message has been sent yet
	ackSent bool
}

type Chunk struct {
	Header *ChunkHeader
	Body   *ChunkData
}

type ChunkHeader struct {
	BasicHeader       *ChunkBasicHeader
	MessageHeader     *ChunkMessageHeader
	ExtendedTimestamp uint32
	// Total elapsed time = timestamp + deltas
	ElapsedTime uint32
}

type ChunkData struct {
	payload []byte
}

type ChunkBasicHeader struct {
	// Chunk type
	FMT           uint8
	ChunkStreamID uint32
}

type ChunkMessageHeader struct {
	// Absolute timestamp of the message (if ChunkHeader.BasicHeader.FMT == 0, chunk type 0), or the timestamp delta if other type of chunk
	Timestamp       uint32
	MessageLength   uint32
	MessageTypeID   uint8
	MessageStreamID uint32
}

func NewChunkHandler(reader *bufio.Reader, writer *bufio.Writer) *ChunkHandler {
	return &ChunkHandler{
		socketr:         reader,
		socketw:         writer,
		inChunkSize:     DefaultMaximumChunkSize,
		outChunkSize:    DefaultMaximumChunkSize,
		ackSent:         false,
		prevChunkHeader: make(map[uint32]ChunkHeader),
	}
}

func (chunkHandler *ChunkHandler) ReadChunkHeader() (ch ChunkHeader, n int, err error) {
	ch = ChunkHeader{}
	r, err := chunkHandler.readBasicHeader(&ch)
	n += r
	if err != nil {
		return ch, n, err
	}
	r, err = chunkHandler.readMessageHeader(&ch)
	n += r
	if err != nil {
		return ch, n, err
	}

	isExtendedTimestamp := false
	// Check if this chunk has an extended timestamp, and if it does then read it. A Timestamp of 0xFFFFFF indicates an extended timestamp.
	if ch.MessageHeader.Timestamp == 0xFFFFFF {
		r, err = chunkHandler.readExtendedTimestamp(&ch)
		n += r
		if err != nil {
			return ch, n, err
		}
		isExtendedTimestamp = true
	}

	csid := ch.BasicHeader.ChunkStreamID

	// If this is a type 0 chunk header, it contains an absolute timestamp. Set the elapsed time to that absolute timestamp.
	// Otherwise add the delta to the elapsed time
	if ch.BasicHeader.FMT == ChunkType0 {
		if isExtendedTimestamp {
			ch.ElapsedTime = ch.ExtendedTimestamp
		} else {
			ch.ElapsedTime = ch.MessageHeader.Timestamp
		}
	} else {
		if isExtendedTimestamp {
			// Handling overflows is unnecessary because Go automatically wraps around
			ch.ElapsedTime = chunkHandler.prevChunkHeader[csid].ElapsedTime + ch.ExtendedTimestamp
		} else {
			// Handling overflows is unnecessary because Go automatically wraps around
			ch.ElapsedTime = chunkHandler.prevChunkHeader[csid].ElapsedTime + ch.MessageHeader.Timestamp
		}
	}

	chunkHandler.prevChunkHeader[csid] = ch
	return ch, n, err
}

func (chunkHandler *ChunkHandler) readBasicHeader(header *ChunkHeader) (n int, err error) {
	basicHeader := &ChunkBasicHeader{}

	b, err := chunkHandler.socketr.ReadByte()
	if err != nil {
		return n, err
	}
	n++
	// Extract chunk type (FMT field) by getting the 2 highest bits (bit 6 and 7 store fmt)
	basicHeader.FMT = uint8(b) >> 6
	// Get the chunk stream ID (first 6 bits, bits 0-5). 0x3F == 0011 1111 in binary (our bit mask to extract the lowest 6 bits)
	csid := b & uint8(0x3F)

	if csid == 0 {
		// if csid is 0, that means we're dealing with chunk basic header 2 (uses 2 bytes). We've already read one before (b), so read the remaining one.
		id, err := chunkHandler.socketr.ReadByte()
		if err != nil {
			return n, err
		}
		n++
		basicHeader.ChunkStreamID = uint32(id) + 64
	} else if csid == 1 {
		// if csid is 1, that means we're dealing with chunk basic header 3 (uses 3 bytes). We've already read one before (b), so read the remaining two.
		id := make([]byte, 2)
		r, err := io.ReadFull(chunkHandler.socketr, id)
		n += r
		if err != nil {
			return n, err
		}
		basicHeader.ChunkStreamID = uint32(binary.BigEndian.Uint16(id)) + 64
		chunkHandler.bytesReceived += 2
	} else {
		// if csid is neither 0 or 1, that means we're dealing with chunk basic header 1 (uses 1 byte). We already read it.
		basicHeader.ChunkStreamID = uint32(csid)
	}

	header.BasicHeader = basicHeader
	return n, err
}

func (chunkHandler *ChunkHandler) readMessageHeader(header *ChunkHeader) (n int, err error) {
	csid := header.BasicHeader.ChunkStreamID
	_, prevChunkExists := chunkHandler.prevChunkHeader[csid]
	mh := &ChunkMessageHeader{}
	switch header.BasicHeader.FMT {
	//0                   1                   2                   3
	//0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	//+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//|                   timestamp                   |message length |
	//+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//|     message length (cont)     |message type id| msg stream id |
	//+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//|           message stream id (cont)            |
	//+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//
	//	Chunk Message Header - Type 0
	case ChunkType0:
		messageHeader := make([]byte, 11)
		// A chunk of type 0 has a message header size of 11 bytes, so read 11 bytes into our messageHeader buffer
		n, err = io.ReadFull(chunkHandler.socketr, messageHeader)
		if err != nil {
			return n, err
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
		return n, err
	//0                   1                   2                   3
	//0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	//+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//|                timestamp delta                |message length |
	//+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//|     message length (cont)     |message type id|
	//+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//
	//	Chunk Message Header - Type 1
	case ChunkType1:
		messageHeader := make([]byte, 7)
		// A chunk of type 1 has a message header size of 7 bytes, so read 7 bytes into our messageHeader buffer
		n, err = io.ReadFull(chunkHandler.socketr, messageHeader)
		if err != nil {
			return n, err
		}
		// Since the timestamp delta field is 3 bytes long, to be able to interpret it as a 32-bit uint we have to add 1 byte at the beginning (3 + 1 byte = 4 bytes == 32-bits)
		// NOTE: this uses the TimestampDelta field, not the Timestamp field (which is only used for chunk type 0)
		mh.Timestamp = binary.BigEndian.Uint32(append([]byte{0x00}, messageHeader[:3]...))
		// Same for the MessageLength field (3 bytes long as well, so add 1 to the beginning)
		mh.MessageLength = binary.BigEndian.Uint32(append([]byte{0x00}, messageHeader[3:6]...))
		// Message type ID is only 1 byte, so read the byte directly
		mh.MessageTypeID = uint8(messageHeader[6])
		// Chunk type 1 message headers don't have a message stream ID. This chunk takes the same message stream ID as the previous chunk.
		if prevChunkExists {
			mh.MessageStreamID = chunkHandler.prevChunkHeader[csid].MessageHeader.MessageStreamID
		}

		header.MessageHeader = mh
		return n, err
	//0                   1                   2
	//0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3
	//+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//|                timestamp delta                |
	//+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//
	//	Chunk Message Header - Type 2
	case ChunkType2:
		messageHeader := make([]byte, 3)
		// A chunk of type 2 has a message header size of 3 bytes, so read 3 bytes into our messageHeader buffer
		n, err = io.ReadFull(chunkHandler.socketr, messageHeader)
		if err != nil {
			return n, err
		}
		// Since the timestamp delta field is 3 bytes long, to be able to interpret it as a 32-bit uint we have to add 1 byte at the beginning (3 + 1 byte = 4 bytes == 32-bits)
		mh.Timestamp = binary.BigEndian.Uint32(append([]byte{0x00}, messageHeader[:3]...))
		if prevChunkExists {
			// Chunk type 2 message headers don't have a message length. This chunk takes the same message length as the previous chunk.
			mh.MessageLength = chunkHandler.prevChunkHeader[csid].MessageHeader.MessageLength
			// Chunk type 2 message headers don't have a message stream ID. This chunk takes the same message stream ID as the previous chunk.
			mh.MessageStreamID = chunkHandler.prevChunkHeader[csid].MessageHeader.MessageStreamID
			// Chunk type 2 message headers don't have a message type ID. This chunk takes the same message type ID as the previous chunk.
			mh.MessageTypeID = chunkHandler.prevChunkHeader[csid].MessageHeader.MessageTypeID
		}

		header.MessageHeader = mh
		return n, err
	case ChunkType3:
		// Chunk type 3 message headers don't have any data. All values are taken from the previous header.
		if prevChunkExists {
			mh.MessageLength = chunkHandler.prevChunkHeader[csid].MessageHeader.MessageLength
			mh.MessageTypeID = chunkHandler.prevChunkHeader[csid].MessageHeader.MessageTypeID
			mh.MessageStreamID = chunkHandler.prevChunkHeader[csid].MessageHeader.MessageStreamID
		}
		header.MessageHeader = mh
		return n, err
	default:
		return n, InvalidChunkType
	}
}

// assembleMessage is called when the length of a message is greater than the currently set chunkSize.
// It returns the final payload of the message assembled from multiple chunks.
func (chunkHandler *ChunkHandler) assembleMessage(messageLength uint32) (payload []byte, n int, err error) {
	//fmt.Println("assembling message...")
	payload = make([]byte, messageLength)
	// Read the initial chunk data that was sent with the first chunk header
	n, err = io.ReadFull(chunkHandler.socketr, payload[:chunkHandler.inChunkSize])
	offset := chunkHandler.inChunkSize
	if err != nil {
		return payload, n, err
	}

	// While there are still more bytes to read
	for offset < messageLength {
		// Read the next chunks (header + data) until we complete our message
		_, r, err := chunkHandler.ReadChunkHeader()
		//fmt.Printf("assembling message, encountered chunk type: %d, message type: %d\n", h.BasicHeader.FMT, h.MessageHeader.MessageTypeID)
		n += r
		if err != nil {
			return payload, n, err
		}
		// If this chunk is still not the end of the message, then read the whole chunk
		if offset+chunkHandler.inChunkSize < messageLength {
			r, err := io.ReadFull(chunkHandler.socketr, payload[offset:offset+chunkHandler.inChunkSize])
			n += r
			if err != nil {
				return payload, n, err
			}
			offset += chunkHandler.inChunkSize
		} else {
			// If this is the last chunk of the message, just read the remaining bytes
			remainingBytes := messageLength - offset
			r, err := io.ReadFull(chunkHandler.socketr, payload[offset:offset+remainingBytes])
			n += r
			if err != nil {
				return payload, n, err
			}
			offset += remainingBytes
		}
	}
	return payload, n, err
}

func (chunkHandler *ChunkHandler) ReadChunkData(header ChunkHeader) (payload []byte, n int, err error) {
	messageLength := header.MessageHeader.MessageLength
	// Check if the length of the message is greater than the chunk size (default chunk size is 128 if no Set Chunk Size message has been received).
	// If it is, we have to assemble the complete message from various chunks.
	if messageLength > chunkHandler.inChunkSize {
		//fmt.Println("assembling message, encountered chunk type:", header.BasicHeader.FMT)
		payload, n, err = chunkHandler.assembleMessage(messageLength)
		//if header.MessageHeader.MessageTypeID == VideoMessage {
		//	fmt.Printf("assembled message of type: %d, message length: %d\n", header.MessageHeader.MessageTypeID, messageLength)
		//}
		if err != nil {
			return payload, n, err
		}
	} else {
		//if header.MessageHeader.MessageTypeID == VideoMessage {
		//	fmt.Printf("read chunk data, no need to assemble message, type: %d, message length: %d\n", header.MessageHeader.MessageTypeID, messageLength)
		//}
		payload = make([]byte, messageLength)
		n, err = io.ReadFull(chunkHandler.socketr, payload)
		if err != nil {
			return payload, n, err
		}
	}

	return payload, n, err
}

func (chunkHandler *ChunkHandler) readExtendedTimestamp(header *ChunkHeader) (n int, err error) {
	extendedTimestamp := make([]byte, 4)
	n, err = io.ReadFull(chunkHandler.socketr, extendedTimestamp)
	if err != nil {
		return
	}
	header.ExtendedTimestamp = binary.BigEndian.Uint32(extendedTimestamp)
	return n, err
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
	// TODO: wrap the socketr in a more user friendly struct that uses Write and Flush in one method
	chunkHandler.socketw.Write(message)
	chunkHandler.socketw.Flush()
}

func (chunkHandler *ChunkHandler) sendSetPeerBandWidth(size uint32, limit uint8) {
	message := generateSetPeerBandwidthMessage(size, limit)
	chunkHandler.socketw.Write(message)
	chunkHandler.socketw.Flush()
}

func (chunkHandler *ChunkHandler) sendBeginStream(streamID uint32) {
	message := generateStreamBeginMessage(streamID)
	chunkHandler.socketw.Write(message)
	chunkHandler.socketw.Flush()
}

func (chunkHandler *ChunkHandler) sendSetChunkSize(size uint32) {
	message := generateSetChunkSizeMessage(size)
	chunkHandler.socketw.Write(message)
	chunkHandler.socketw.Flush()
	chunkHandler.outChunkSize = size
}

func (chunkHandler *ChunkHandler) sendConnectSuccess(csID uint32) {
	message := generateConnectResponseSuccess(csID)
	chunkHandler.socketw.Write(message)
	chunkHandler.socketw.Flush()
}

func (chunkHandler *ChunkHandler) sendAck() {
	message := generateAckMessage(chunkHandler.bytesReceived)
	chunkHandler.socketw.Write(message)
	chunkHandler.socketw.Flush()
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
	_, err := chunkHandler.socketw.Write(header)
	if err != nil {
		return err
	}

	// Determine if we have to chunk our payload
	if len(payload) > int(chunkHandler.outChunkSize) {
		payloadLength := len(payload)
		// take whatever csid came in the original header, and use it for future chunks. Also specify fmt = 3 (chunk header - type 3) for subsequent chunks
		chunk3Header := (ChunkType3 << 6) | (header[0] & 0x3F)

		chunkSize := int(chunkHandler.outChunkSize)
		bytesWritten := 0 // bytes of the PAYLOAD we've written
		// True if this is the first time we're going to write payload data in a chunk
		firstPayloadChunk := true
		for bytesWritten < payloadLength {
			if !firstPayloadChunk {
				// We've already written payload data, so separate it with a chunk type 3 header
				err = chunkHandler.socketw.WriteByte(chunk3Header)
				if err != nil {
					return err
				}
			} else {
				firstPayloadChunk = false
			}
			// if the next chunk is still not the end of the message, write chunk size bytes
			if bytesWritten+chunkSize < payloadLength {
				_, err = chunkHandler.socketw.Write(payload[bytesWritten : bytesWritten+chunkSize])
				if err != nil {
					return err
				}
				bytesWritten += chunkSize
			} else {
				// Write remaining data
				remainingBytes := payloadLength - bytesWritten
				_, err = chunkHandler.socketw.Write(payload[bytesWritten : bytesWritten+remainingBytes])
				bytesWritten += remainingBytes
			}
		}
	} else {
		// No chunking needed
		_, err := chunkHandler.socketw.Write(payload)
		if err != nil {
			return err
		}
	}

	err = chunkHandler.socketw.Flush()
	if err != nil {
		return err
	}
	return nil
}

func (chunkHandler *ChunkHandler) sendBytes(bytes []byte) (n int, err error) {
	n, err = chunkHandler.socketw.Write(bytes)
	if err != nil {
		return
	}
	err = chunkHandler.socketw.Flush()
	return
}
