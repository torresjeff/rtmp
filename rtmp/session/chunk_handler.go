package session

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"github.com/pkg/errors"
	"github.com/torresjeff/rtmp-server/amf/amf0"
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

const LimitHard uint8 = 0
const LimitSoft uint8 = 1
const LimitDynamic uint8 = 2

type ChunkHandler struct {
	socket          *bufio.ReadWriter
	prevChunkHeader *ChunkHeader
	inChunkSize     uint32
	windowAckSize   uint32
	bytesReceived   uint32
	outBandwidth    uint32
	limit           uint8

	// app data
	app string
	flashVer string
	swfUrl string
	tcUrl string
	typeOfStream string
	streamKey string // used to identify user
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

func NewChunkHandler(reader *bufio.ReadWriter) *ChunkHandler {
	return &ChunkHandler{
		socket:      reader,
		inChunkSize: DefaultMaximumChunkSize,
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

func (chunkHandler *ChunkHandler) ReadChunkData(header *ChunkHeader) (*ChunkData, error) {
	switch header.MessageHeader.MessageTypeID {
	case SetChunkSize, AbortMessage, Ack, WindowAckSize, SetPeerBandwidth:
		return chunkHandler.handleControlMessage(header)
	case UserControlMessage:
		return chunkHandler.handleUserControlMessage(header)
	case CommandMessageAMF0, CommandMessageAMF3:
		fmt.Println("received command message")
		return chunkHandler.handleCommandMessage(header.MessageHeader.MessageTypeID, header.MessageHeader.MessageLength)
	}

	return nil, nil
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

	chunkHandler.updateBytesReceived(1)

	if csid == 0 {
		// if csid is 0, that means we're dealing with chunk basic header 2 (uses 2 bytes)
		id, err := chunkHandler.socket.ReadByte()
		if err != nil {
			return err
		}
		basicHeader.ChunkStreamID = uint32(id) + 64
		chunkHandler.updateBytesReceived(uint32(1))
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
		// if csid is neither 0 or 1, that means we're dealing with chunk basic header 1 (uses 1 byte). This represents the actual chunk stream ID.
		basicHeader.ChunkStreamID = uint32(csid)
	}

	header.BasicHeader = basicHeader
	return nil
}

func (chunkHandler *ChunkHandler) readMessageHeader(header *ChunkHeader) error {
	mh := &ChunkMessageHeader{}
	switch header.BasicHeader.FMT {
	case ChunkType0:
		if config.Debug {
			fmt.Println("chunk type 0 detected")
		}
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
		// Finally, read the message stream id (remaining 4 bytes)
		// NOTE: message stream ID is stored in little endian format
		mh.MessageStreamID = binary.LittleEndian.Uint32(messageHeader[7:])

		header.MessageHeader = mh

		return nil
	case ChunkType1:
		if config.Debug {
			fmt.Println("chunk type 1 detected")
		}
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
		//mh.MessageStreamID = chunk.prevChunkHeader.MessageHeader.MessageStreamID

		header.MessageHeader = mh
		return nil
	case ChunkType2:
		if config.Debug {
			fmt.Println("chunk type 2 detected")
		}
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
		//mh.MessageLength = chunk.prevChunkHeader.MessageHeader.MessageLength
		// Chunk type 2 message headers don't have a message stream ID. This chunk takes the same message stream ID as the previous chunk.
		//mh.MessageStreamID = chunk.prevChunkHeader.MessageHeader.MessageStreamID
		// Chunk type 2 message headers don't have a message type ID. This chunk takes the same message type ID as the previous chunk.
		//mh.MessageTypeID = chunk.prevChunkHeader.MessageHeader.MessageTypeID

		header.MessageHeader = mh
		return nil
	case ChunkType3:
		if config.Debug {
			fmt.Println("chunk type 3 detected")
		}
		// Chunk type 3 message headers don't have any data. All values are taken from the previous header.

		// As per the spec: If a Type 3 chunk follows a Type 0 chunk, is this only useful for then the timestamp delta for this Type 3 chunk is the same as the timestamp of the Type 0 chunk.
		// This would be handled by the RTMP client, though, to form audio and video messages. Since we aren't doing anything with the messages (just forwarding them to the client),
		// we assume that the client will keep track of the previous chunk header.
		//if chunk.prevChunkHeader.BasicHeader.FMT == ChunkType0 {
		//	mh.TimestampDelta = chunk.prevChunkHeader.MessageHeader.Timestamp
		//} else {
		//	mh.TimestampDelta = chunk.prevChunkHeader.MessageHeader.TimestampDelta
		//}
		//mh.MessageLength = chunk.prevChunkHeader.MessageHeader.MessageLength
		//mh.MessageTypeID = chunk.prevChunkHeader.MessageHeader.MessageTypeID
		//mh.MessageStreamID = chunk.prevChunkHeader.MessageHeader.MessageStreamID
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

func (chunkHandler *ChunkHandler) handleControlMessage(header *ChunkHeader) (*ChunkData, error) {
	messageLength := header.MessageHeader.MessageLength
	switch header.MessageHeader.MessageTypeID {
	case SetChunkSize:
		if config.Debug {
			fmt.Println("Received SetChunkSize control message")
		}
		// The payload of a set chunk size message is the new chunk size
		// TODO: what if message length is greater than inChunkSize? in this case, the chunk has to be assembled to be able to interpret it
		// TODO: for now, assume that all messages fit in a chunk
		chunkSize := make([]byte, messageLength)
		_, err := io.ReadAtLeast(chunkHandler.socket, chunkSize, int(messageLength))
		if err != nil {
			return nil, err
		}
		chunkHandler.inChunkSize = binary.BigEndian.Uint32(chunkSize)
		if config.Debug {
			fmt.Println("Set inChunkSize to", chunkHandler.inChunkSize)
		}

		return &ChunkData{
			payload: chunkSize,
		}, nil
	case AbortMessage:
		// The payload of an abort message is the chunk stream ID whose current message is to be discarded
		chunkStreamId := make([]byte, messageLength)
		_, err := io.ReadAtLeast(chunkHandler.socket, chunkStreamId, int(messageLength))
		if err != nil {
			return nil, err
		}
		return &ChunkData{
			payload: chunkStreamId,
		}, nil
	case Ack:
		// The payload of an ack message is the sequence number (number of bytes received so far)
		sequenceNumber := make([]byte, messageLength)
		_, err := io.ReadAtLeast(chunkHandler.socket, sequenceNumber, int(messageLength))
		if err != nil {
			return nil, err
		}
		return &ChunkData{
			payload: sequenceNumber,
		}, nil
	case WindowAckSize:
		// The payload of a window ack size is the window size to use between acknowledgements
		ackWindowSize := make([]byte, messageLength)
		_, err := io.ReadAtLeast(chunkHandler.socket, ackWindowSize, int(messageLength))
		if err != nil {
			return nil, err
		}

		// the ack window size is in the first 4 bytes
		chunkHandler.windowAckSize = binary.BigEndian.Uint32(ackWindowSize[:4])

		return &ChunkData{
			payload: ackWindowSize,
		}, nil
	case SetPeerBandwidth:
		// The payload of a set peer bandwidth message is the window size and the limit type (hard, soft, dynamic)
		ackWindowSize := make([]byte, messageLength)
		_, err := io.ReadAtLeast(chunkHandler.socket, ackWindowSize, int(messageLength))
		if err != nil {
			return nil, err
		}
		//the ack window size is in the first 4 bytes
		windowAckSize := binary.BigEndian.Uint32(ackWindowSize[:4])
		// The peer receiving this message SHOULD respond with a Window Acknowledgement Size message if the window size
		// is different from the last one sent to the sender of this message.
		if chunkHandler.windowAckSize != windowAckSize {
			chunkHandler.sendAck()
			chunkHandler.windowAckSize = windowAckSize
		}

		// for now, ignore the limit type in byte 5
		//limitType := ackWindowSize[4:]

		return &ChunkData{
			payload: ackWindowSize,
		}, nil
	default:
		return nil, errors.New("received unsupported message type ID")
	}
}

func (chunkHandler *ChunkHandler) handleUserControlMessage(header *ChunkHeader) (*ChunkData, error) {
	messageLength := header.MessageHeader.MessageLength
	// The first 2 bytes of the payload define the event type, the rest of the payload is the event data (the size of event data is variable)
	payload := make([]byte, messageLength)
	_, err := io.ReadAtLeast(chunkHandler.socket, payload, int(messageLength))
	if err != nil {
		return nil, err
	}
	return &ChunkData{
		payload: payload,
	}, nil
}

func (chunkHandler *ChunkHandler) updateBytesReceived(i uint32) {
	chunkHandler.bytesReceived += i
	// TODO: implement send ack
	if chunkHandler.bytesReceived >= chunkHandler.windowAckSize {
		chunkHandler.sendAck()
		// Reset the number of bytes received
		chunkHandler.bytesReceived = 0
	}
}

func (chunkHandler *ChunkHandler) sendAck() {
	// TODO: implement send the acknowledgemnent
}

func (chunkHandler *ChunkHandler) handleCommandMessage(commandType uint8, messageLength uint32) (*ChunkData, error) {
	payload := make([]byte, messageLength)
	_, err := io.ReadAtLeast(chunkHandler.socket, payload, int(messageLength))
	if err != nil {
		return nil, err
	}

	switch commandType {
	case CommandMessageAMF0:
		commandName, err := amf0.Decode(payload) // Decode the command name (always the first string in the payload)
		if err != nil {
			return nil, err
		}

		chunkHandler.handleCommandAmf0(commandName.(string), payload[amf0.Size(commandName.(string)):])
		return &ChunkData{
			payload: payload,
		}, nil
	case CommandMessageAMF3:
		// TODO: implement AMF3
	}

	return nil, nil
}

func (chunkHandler *ChunkHandler) handleCommandAmf0(commandName string, payload []byte) {
	switch commandName {
	case "connect":
		fmt.Println("received connect command")
		transactionId, _ := amf0.Decode(payload)
		byteLength := amf0.Size(transactionId.(float64))
		// Update our payload to read the next property (commandObject)
		payload = payload[byteLength:]
		commandObject, _ := amf0.Decode(payload)
		if config.Debug {
			fmt.Println(fmt.Sprintf("received command connect with transactionId %f", transactionId.(float64)))
			fmt.Println(fmt.Sprintf("received command connect with commandObject %+v", commandObject.(map[string]interface{})))
		}

		// TODO: should this data be stored in the chunkHandler or in the session?
		commandObjectMap := commandObject.(map[string]interface{})

		// Playback clients send other properties in the command object, such as what audio/video codecs the client supports
		chunkHandler.app = commandObjectMap["app"].(string)
		if _, exists := commandObjectMap["flashVer"]; exists {
			chunkHandler.flashVer = commandObjectMap["flashVer"].(string)
		} else if _, exists := commandObjectMap["flashver"]; exists {
			chunkHandler.flashVer = commandObjectMap["flashver"].(string)
		}
		chunkHandler.swfUrl = commandObjectMap["swfUrl"].(string)
		chunkHandler.tcUrl = commandObjectMap["tcUrl"].(string)
		chunkHandler.typeOfStream = commandObjectMap["type"].(string)

		// If the app name to connect  is PublishApp (whatever the user specifies in the config, ie. "app", "app/publish"),
		// this means the user wants to stream, follow the flow to start a stream
		// TODO: is this specific for publishing? or does this apply for consuming clients as well?
		if chunkHandler.app == config.PublishApp {
			// As per the specification, after the connect command, the server sends the protocol message Window Acknowledgment Size
			chunkHandler.sendWindowAckSize(config.DefaultClientWindowSize)
			// TODO: connect to the app (whatever that is)
			// After sending the window ack size message, the server sends the set peer bandwidth message
			chunkHandler.sendSetPeerBandWidth(config.DefaultClientWindowSize, LimitDynamic)
			// Send the User Control Message to begin stream with stream ID DefaultPublishStream (which is 0)
			chunkHandler.sendBeginStream(config.DefaultPublishStream)
			// Send Set Chunk Size message
			chunkHandler.sendSetChunkSize(config.DefaultChunkSize)
			// Send Connect Success response
			chunkHandler.sendConnectSuccess()
		} else {
			fmt.Println("user trying to connect to app", chunkHandler.app, ", but the app doesn't exist")
		}


	}
}

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
}

func (chunkHandler *ChunkHandler) sendConnectSuccess() {
	message := generateConnectResponseSuccess()
	chunkHandler.socket.Write(message)
	chunkHandler.socket.Flush()
}