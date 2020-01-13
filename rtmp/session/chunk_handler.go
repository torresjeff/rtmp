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

type surroundSound struct {
	stereoSound bool
	twoPointOneSound bool
	threePointOneSound bool
	fourPointZeroSound bool
	fourPointOneSound bool
	fivePointOneSound bool
	sevenPointOneSound bool
}

type clientMetadata struct {
	duration float64
	fileSize float64
	width float64
	height float64
	videoCodecID string
	videoDataRate float64
	frameRate float64
	audioCodecID string
	audioDataRate float64
	audioSampleRate float64
	audioSampleSize float64
	audioChannels float64
	surroundSound surroundSound
	encoder string
}

type ChunkHandler struct {
	socket          *bufio.ReadWriter
	prevChunkHeader *ChunkHeader
	inChunkSize     uint32
	windowAckSize   uint32
	bytesReceived   uint32
	outBandwidth    uint32
	limit           uint8
	clientMetadata  clientMetadata

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
	fmt.Println("reading chunk header")
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

func (chunkHandler *ChunkHandler) ReadChunkData(header *ChunkHeader) (*ChunkData, error) {
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
	switch header.MessageHeader.MessageTypeID {
	case SetChunkSize, AbortMessage, Ack, WindowAckSize, SetPeerBandwidth:
		return chunkHandler.handleControlMessage(header, payload)
	case UserControlMessage:
		return chunkHandler.handleUserControlMessage(header, payload)
	case CommandMessageAMF0, CommandMessageAMF3:
		fmt.Println("received command message")
		return chunkHandler.handleCommandMessage(header.BasicHeader.ChunkStreamID, header.MessageHeader.MessageStreamID, header.MessageHeader.MessageTypeID, header.MessageHeader.MessageLength, payload)
	case DataMessageAMF0, DataMessageAMF3:
		return chunkHandler.handleDataMessage(header.MessageHeader.MessageTypeID, header.MessageHeader.MessageLength, payload)
	case AudioMessage, VideoMessage:
		// TODO: handle audio and video messages. For now just read the payload and continue

	default:
		fmt.Println("received unknown message header")
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
		mh.MessageStreamID = chunkHandler.prevChunkHeader.MessageHeader.MessageStreamID

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
		mh.MessageLength = chunkHandler.prevChunkHeader.MessageHeader.MessageLength
		// Chunk type 2 message headers don't have a message stream ID. This chunk takes the same message stream ID as the previous chunk.
		mh.MessageStreamID = chunkHandler.prevChunkHeader.MessageHeader.MessageStreamID
		// Chunk type 2 message headers don't have a message type ID. This chunk takes the same message type ID as the previous chunk.
		mh.MessageTypeID = chunkHandler.prevChunkHeader.MessageHeader.MessageTypeID

		header.MessageHeader = mh
		return nil
	case ChunkType3:
		if config.Debug {
			fmt.Println("chunk type 3 detected")
		}
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

func (chunkHandler *ChunkHandler) handleControlMessage(header *ChunkHeader, payload []byte) (*ChunkData, error) {
	switch header.MessageHeader.MessageTypeID {
	case SetChunkSize:
		if config.Debug {
			fmt.Println("Received SetChunkSize control message")
		}
		// The payload of a set chunk size message is the new chunk size
		// TODO: what if message length is greater than inChunkSize? in this case, the chunk has to be assembled to be able to interpret it
		// TODO: for now, assume that all messages fit in a chunk
		chunkHandler.inChunkSize = binary.BigEndian.Uint32(payload)
		if config.Debug {
			fmt.Println("Set inChunkSize to", chunkHandler.inChunkSize)
		}

		return &ChunkData{
			payload: payload,
		}, nil
	case AbortMessage:
		// The payload of an abort message is the chunk stream ID whose current message is to be discarded
		// TODO: implement abort
		return &ChunkData{
			payload: payload,
		}, nil
	case Ack:
		// The payload of an ack message is the sequence number (number of bytes received so far)
		return &ChunkData{
			payload: payload,
		}, nil
	case WindowAckSize:
		// the ack window size is in the first 4 bytes
		chunkHandler.windowAckSize = binary.BigEndian.Uint32(payload[:4])

		return &ChunkData{
			payload: payload,
		}, nil
	case SetPeerBandwidth:
		// window ack size is in the first 4 bytes
		windowAckSize := binary.BigEndian.Uint32(payload[:4])
		// The peer receiving this message SHOULD respond with a Window Acknowledgement Size message if the window size
		// is different from the last one sent to the sender of this message.
		if chunkHandler.windowAckSize != windowAckSize {
			chunkHandler.sendAck()
			chunkHandler.windowAckSize = windowAckSize
		}

		// for now, ignore the limit type in byte 5
		//limitType := ackWindowSize[4:]

		return &ChunkData{
			payload: payload,
		}, nil
	default:
		return nil, errors.New("received unsupported message type ID")
	}
}

func (chunkHandler *ChunkHandler) handleUserControlMessage(header *ChunkHeader, payload []byte) (*ChunkData, error) {
	// TODO: handle the user control message

	return &ChunkData{
		payload: payload,
	}, nil
}

func (chunkHandler *ChunkHandler) updateBytesReceived(i uint32) {
	chunkHandler.bytesReceived += i
	// TODO: implement send ack
	if chunkHandler.bytesReceived >= chunkHandler.windowAckSize {
		chunkHandler.sendAck()
	}
}

func (chunkHandler *ChunkHandler) handleCommandMessage(csID uint32, streamID uint32, commandType uint8, messageLength uint32, payload []byte) (*ChunkData, error) {
	//payload := make([]byte, messageLength)
	//_, err := io.ReadAtLeast(chunkHandler.socket, payload, int(messageLength))
	//if err != nil {
	//	return nil, err
	//}

	switch commandType {
	case CommandMessageAMF0:
		commandName, err := amf0.Decode(payload) // Decode the command name (always the first string in the payload)
		if err != nil {
			return nil, err
		}

		chunkHandler.handleCommandAmf0(csID, streamID, commandName.(string), payload[amf0.Size(commandName.(string)):])
		return &ChunkData{
			payload: payload,
		}, nil
	case CommandMessageAMF3:
		// TODO: implement AMF3
	}

	return nil, nil
}

func (chunkHandler *ChunkHandler) handleCommandAmf0(csID uint32, streamID uint32, commandName string, payload []byte) {
	switch commandName {
	case "connect":
		transactionId, _ := amf0.Decode(payload)
		byteLength := amf0.Size(transactionId)
		// Update our payload to read the next property (commandObject)
		payload = payload[byteLength:]
		commandObject, _ := amf0.Decode(payload)
		if config.Debug {
			fmt.Println(fmt.Sprintf("received command connect with transactionId %f", transactionId.(float64)))
			fmt.Println(fmt.Sprintf("received command connect with commandObject %+v", commandObject))
		}
		chunkHandler.onConnect(csID, transactionId.(float64), commandObject.(map[string]interface{}))
	case "releaseStream":
		transactionId, _ := amf0.Decode(payload)
		// Update our payload to read the next property (NULL)
		byteLength := amf0.Size(transactionId)
		payload = payload[byteLength:]
		null, _ := amf0.Decode(payload)
		// Update our payload to read the next property (stream key)
		byteLength = amf0.Size(null)
		payload = payload[byteLength:]
		streamKey, _ := amf0.Decode(payload)
		chunkHandler.onReleaseStream(csID, transactionId.(float64), streamKey.(string))
	case "FCPublish":
		transactionId, _ := amf0.Decode(payload)
		// Update our payload to read the next property (NULL)
		byteLength := amf0.Size(transactionId)
		payload = payload[byteLength:]
		null, _ := amf0.Decode(payload)
		// Update our payload to read the next property (stream key)
		byteLength = amf0.Size(null)
		payload = payload[byteLength:]
		streamKey, _ := amf0.Decode(payload)
		chunkHandler.onFCPublish(csID, transactionId.(float64), streamKey.(string))
	case "createStream":
		transactionId, _ := amf0.Decode(payload)
		byteLength := amf0.Size(transactionId)
		// Update our payload to read the next property (commandObject)
		payload = payload[byteLength:]
		commandObject, _ := amf0.Decode(payload)
		if config.Debug {
			fmt.Println(fmt.Sprintf("received command createStream with transactionId %f", transactionId.(float64)))
			fmt.Println(fmt.Sprintf("received command createStream with commandObject %+v", commandObject))
		}
		// Handle cases where the command object that was sent was null
		switch commandObject.(type) {
		case nil:
			chunkHandler.onCreateStream(csID, transactionId.(float64), nil)
		default:
			chunkHandler.onCreateStream(csID, transactionId.(float64), commandObject.(map[string]interface{}))
		}
	case "publish":
		transactionId, _ := amf0.Decode(payload)
		byteLength := amf0.Size(transactionId)
		// Update our payload to read the next property (commandObject)
		payload = payload[byteLength:]
		// Command object is set to null for the publish command
		commandObject, _ := amf0.Decode(payload)
		byteLength = amf0.Size(commandObject)
		payload = payload[byteLength:]
		// name with which the stream is published (basically the streamKey)
		streamKey, _ := amf0.Decode(payload)
		byteLength = amf0.Size(streamKey)
		payload = payload[byteLength:]
		// Publishing type: "live", "record", or "append"
		// - record: The stream is published and the data is recorded to a new file. The file is stored on the server
		// in a subdirectory within the directory that contains the server application. If the file already exists, it is overwritten.
		// - append: The stream is published and the data is appended to a file. If no file is found, it is created.
		// - live: Live data is published without recording it in a file.
		publishingType, _ := amf0.Decode(payload)
		chunkHandler.onPublish(csID, streamID, transactionId.(float64), streamKey.(string), publishingType.(string))
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

func (chunkHandler *ChunkHandler) sendConnectSuccess(csID uint32) {
	message := generateConnectResponseSuccess(csID)
	chunkHandler.socket.Write(message)
	chunkHandler.socket.Flush()
}

func (chunkHandler *ChunkHandler) sendAck() {
	// TODO: implement send the acknowledgemnent

	// Reset the number of bytes received
	chunkHandler.bytesReceived = 0
}

func (chunkHandler *ChunkHandler) storeMetadata(metadata map[string]interface{}) {
	// Playback clients send other properties in the command object, such as what audio/video codecs the client supports
	// TODO: should this data be stored in the chunkHandler or in the session?
	chunkHandler.app = metadata["app"].(string)
	if _, exists := metadata["flashVer"]; exists {
		chunkHandler.flashVer = metadata["flashVer"].(string)
	} else if _, exists := metadata["flashver"]; exists {
		chunkHandler.flashVer = metadata["flashver"].(string)
	}

	if _, exists := metadata["swfUrl"]; exists {
		chunkHandler.flashVer = metadata["swfUrl"].(string)
	} else if _, exists := metadata["swfurl"]; exists {
		chunkHandler.flashVer = metadata["swfurl"].(string)
	}

	if _, exists := metadata["tcUrl"]; exists {
		chunkHandler.flashVer = metadata["tcUrl"].(string)
	} else if _, exists := metadata["tcurl"]; exists {
		chunkHandler.flashVer = metadata["tcurl"].(string)
	}

	if _, exists := metadata["type"]; exists {
		chunkHandler.flashVer = metadata["type"].(string)
	}
}

func (chunkHandler *ChunkHandler) onConnect(csID uint32, transactionID float64, commandObject map[string]interface{}) {
	fmt.Println("received connect command")
	chunkHandler.storeMetadata(commandObject)
	// If the app name to connect  is PublishApp (whatever the user specifies in the config, ie. "app", "app/publish"),
	// this means the user wants to stream, follow the flow to start a stream
	// TODO: is this specific for publishing? or does this apply for consuming clients as well?
	if chunkHandler.app == config.PublishApp {
		// As per the specification, after the connect command, the server sends the protocol message Window Acknowledgment Size
		chunkHandler.sendWindowAckSize(config.DefaultClientWindowSize)
		// TODO: connect to the app (whatever that is)
		// After sending the window ack size message, the server sends the set peer bandwidth message
		chunkHandler.sendSetPeerBandWidth(config.DefaultClientWindowSize, LimitDynamic)
		// Send the User Control Message to begin stream with stream ID = DefaultPublishStream (which is 0)
		// Subsequent messages sent by the client will have stream ID = DefaultPublishStream, until another sendBeginStream message is sent
		chunkHandler.sendBeginStream(config.DefaultPublishStream)
		// Send Set Chunk Size message
		chunkHandler.sendSetChunkSize(config.DefaultChunkSize)
		// Send Connect Success response
		chunkHandler.sendConnectSuccess(csID)
	} else {
		fmt.Println("user trying to connect to app", chunkHandler.app, ", but the app doesn't exist")
	}
}

func (chunkHandler *ChunkHandler) onReleaseStream(csID uint32, transactionID float64, streamKey string) {
	// TODO: what does releaseStream actually do? Does it close the stream?
	// TODO: check stuff like is stream key valid, is the user allowed to release the stream

	// TODO: implement
}

func (chunkHandler *ChunkHandler) onFCPublish(csID uint32, transactionID float64, streamKey string) {
	// TODO: check stuff like is stream key valid, is the user allowed to publish the stream

	chunkHandler.sendOnFCPublish(csID, transactionID, streamKey)
}

func (chunkHandler *ChunkHandler) sendOnFCPublish(csID uint32, transactionID float64, streamKey string) {
	message := generateOnFCPublishMessage(csID, transactionID, streamKey)
	chunkHandler.socket.Write(message)
	chunkHandler.socket.Flush()
}

func (chunkHandler *ChunkHandler) onCreateStream(csID uint32, transactionID float64, commandObject map[string]interface{}) {
	message := generateCreateStreamResponse(csID, transactionID, commandObject)
	chunkHandler.socket.Write(message)
	chunkHandler.socket.Flush()

	chunkHandler.sendBeginStream(uint32(config.DefaultStreamID))
}

func (chunkHandler *ChunkHandler) onPublish(csID uint32, streamID uint32, transactionID float64, streamKey string, publishingType string) {
	// TODO: Handle things like look up the user's stream key, check if it's valid.
	// TODO: For example: twitch returns "Publishing live_user_<username>" in the description.
	// TODO: Handle things like recording into a file if publishingType = "record" or "append"
	// infoObject should have at least three properties: level, code, and description. But may contain other properties.
	infoObject := map[string]interface{}{
		"level": "status",
		"code": "NetStream.Publish.Start",
		"description": "Publishing live_user_<x>",
	}
	// TODO: the transaction ID for onStatus messages should be 0 as per the spec. But twitch sends the transaction ID that was in the request to "publish".
	// For now, reply with the same transaction ID.
	message := generateStatusMessage(transactionID, streamID, infoObject)
	chunkHandler.socket.Write(message)
	chunkHandler.socket.Flush()
}

func (chunkHandler *ChunkHandler) handleDataMessage(dataType uint8, messageLength uint32, payload []byte) (*ChunkData, error) {
	switch dataType {
	case DataMessageAMF0:
		dataName, err := amf0.Decode(payload) // Decode the command name (always the first string in the payload)
		if err != nil {
			return nil, err
		}

		chunkHandler.handleDataMessageAmf0(dataName.(string), payload[amf0.Size(dataName.(string)):])
		return &ChunkData{
			payload: payload,
		}, nil
	case DataMessageAMF3:
		// TODO: implement AMF3
	}
	return nil, nil
}

func (chunkHandler *ChunkHandler) handleDataMessageAmf0(dataName string, payload []byte) {
	switch dataName {
	case "@setDataFrame":
		// @setDataFrame message includes a string with value "onMetadata".
		// Ignore it for now.
		onMetadata, _ := amf0.Decode(payload)
		payload = payload[amf0.Size(onMetadata):]
		// Metadata is sent as an ECMAArray
		metadata, _ := amf0.Decode(payload)
		chunkHandler.setClientMetadata(metadata.(amf0.ECMAArray))
		fmt.Printf("clientMetadata %+v", chunkHandler.clientMetadata)
	}
}

func (chunkHandler *ChunkHandler) setClientMetadata(obj amf0.ECMAArray) {
	if val, exists := obj["duration"]; exists {
		chunkHandler.clientMetadata.duration = val.(float64)
	}
	if val, exists := obj["fileSize"]; exists {
		chunkHandler.clientMetadata.fileSize = val.(float64)
	}
	if val, exists := obj["width"]; exists {
		chunkHandler.clientMetadata.width = val.(float64)
	}
	if val, exists := obj["height"]; exists {
		chunkHandler.clientMetadata.height = val.(float64)
	}
	if val, exists := obj["videocodecid"]; exists {
		chunkHandler.clientMetadata.videoCodecID = val.(string)
	}
	if val, exists := obj["videodatarate"]; exists {
		chunkHandler.clientMetadata.videoDataRate = val.(float64)
	}
	if val, exists := obj["framerate"]; exists {
		chunkHandler.clientMetadata.frameRate = val.(float64)
	}
	if val, exists := obj["audiocodecid"]; exists {
		chunkHandler.clientMetadata.audioCodecID = val.(string)
	}
	if val, exists := obj["audiodatarate"]; exists {
		chunkHandler.clientMetadata.audioDataRate = val.(float64)
	}
	if val, exists := obj["audiosamplerate"]; exists {
		chunkHandler.clientMetadata.audioSampleRate = val.(float64)
	}
	if val, exists := obj["audiosamplesize"]; exists {
		chunkHandler.clientMetadata.audioSampleSize = val.(float64)
	}
	if val, exists := obj["audiochannels"]; exists {
		chunkHandler.clientMetadata.audioChannels = val.(float64)
	}
	if val, exists := obj["stereo"]; exists {
		chunkHandler.clientMetadata.surroundSound.stereoSound = val.(bool)
	}
	if val, exists := obj["2.1"]; exists {
		chunkHandler.clientMetadata.surroundSound.twoPointOneSound = val.(bool)
	}
	if val, exists := obj["3.1"]; exists {
		chunkHandler.clientMetadata.surroundSound.threePointOneSound = val.(bool)
	}
	if val, exists := obj["4.0"]; exists {
		chunkHandler.clientMetadata.surroundSound.fourPointZeroSound = val.(bool)
	}
	if val, exists := obj["4.1"]; exists {
		chunkHandler.clientMetadata.surroundSound.fourPointOneSound = val.(bool)
	}
	if val, exists := obj["5.1"]; exists {
		chunkHandler.clientMetadata.surroundSound.fivePointOneSound = val.(bool)
	}
	if val, exists := obj["7.1"]; exists {
		chunkHandler.clientMetadata.surroundSound.sevenPointOneSound = val.(bool)
	}
	if val, exists := obj["encoder"]; exists {
		chunkHandler.clientMetadata.encoder = val.(string)
	}

}