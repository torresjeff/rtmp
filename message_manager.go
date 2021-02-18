package rtmp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/torresjeff/rtmp/amf/amf0"
	"github.com/torresjeff/rtmp/audio"
	"github.com/torresjeff/rtmp/config"
	"github.com/torresjeff/rtmp/video"
)

// Control message types
const (
	// Control messages MUST have message stream ID 0 and be sent in chunk stream ID 2
	SetChunkSize     = 1
	AbortMessage     = 2
	Ack              = 3
	WindowAckSize    = 5
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

	AudioMessage     = 8
	VideoMessage     = 9
	AggregateMessage = 22
)

const (
	EventStreamBegin uint16 = 0
)

type MessageManager struct {
	session      MediaServer
	handshaker   *Handshaker
	chunkHandler *ChunkHandler
	streamID     uint32
}

func NewMessageManager(session MediaServer, handshaker *Handshaker, chunkHandler *ChunkHandler) *MessageManager {
	return &MessageManager{
		session:      session,
		handshaker:   handshaker,
		chunkHandler: chunkHandler,
	}
}

// Initialize performs the handshake with the client. It returns an error if the handshake was not successful.
// Initialize should not be called again for the remainder of the session. Calling Initialize more than once will result
// in an error.
// This method is used for servers only.
func (m *MessageManager) Initialize() error {
	return m.handshaker.Handshake()
}

// InitializeClient performs the handshake with the server. It returns an error if the handshake was not successful.
// InitializeClient should not be called again for the remainder of the session. Calling InitializeClient more than once
// will result in an error.
// This method is used for clients only.
func (m *MessageManager) InitializeClient() error {
	return m.handshaker.ClientHandshake()
}

// Reads the next chunk header + data
func (m *MessageManager) nextMessage() error {
	// TODO: every time a chunk is read, update the number of read bytes
	var err error
	chunkHeader, _, err := m.chunkHandler.ReadChunkHeader()
	if err != nil {
		return err
	}

	payload, _, err := m.chunkHandler.ReadChunkData(chunkHeader)
	if err != nil {
		return err
	}

	return m.interpretMessage(chunkHeader, payload)
}

func (m *MessageManager) interpretMessage(header ChunkHeader, payload []byte) error {
	// calculate timestamp from headers. Useful for audio/video messages
	// Header has an extended timestamp
	//var timestamp uint32
	//if header.MessageHeader.Timestamp == 0xFFFFFF || header.MessageHeader.TimestampDelta == 0xFFFFFF {
	//	timestamp = header.ExtendedTimestamp
	//} else if header.BasicHeader.FMT == ChunkType0 {
	//	timestamp = header.MessageHeader.Timestamp
	//} else {
	//	timestamp = header.MessageHeader.TimestampDelta
	//}
	//fmt.Printf("basic header %+v, message header %+v", header.BasicHeader, header.MessageHeader)
	switch header.MessageHeader.MessageTypeID {
	case SetChunkSize, AbortMessage, Ack, WindowAckSize, SetPeerBandwidth:
		return m.handleControlMessage(&header, payload)
	case UserControlMessage:
		// First 2 bytes of payload contain event type
		eventType := binary.BigEndian.Uint16(payload[:2])
		return m.handleUserControlMessage(&header, eventType, payload[2:])
	case CommandMessageAMF0, CommandMessageAMF3:
		return m.handleCommandMessage(header.BasicHeader.ChunkStreamID, header.MessageHeader.MessageStreamID, header.MessageHeader.MessageTypeID, payload)
	case DataMessageAMF0, DataMessageAMF3:
		return m.handleDataMessage(header.MessageHeader.MessageTypeID, payload)
	case AudioMessage:
		//fmt.Print(" audio\n")
		//fmt.Printf("audio message: fmt %d, chunk stream id %d, message stream id %d, timestamp %d, elapsed time %d, message length %d\n", header.BasicHeader.FMT, header.BasicHeader.ChunkStreamID,
		//	header.MessageHeader.MessageStreamID, header.MessageHeader.Timestamp, header.ElapsedTime, header.MessageHeader.MessageLength)
		return m.handleAudioMessage(header.BasicHeader.ChunkStreamID, header.MessageHeader.MessageStreamID, payload, header.ElapsedTime)
	case VideoMessage:
		//fmt.Print(" video\n")
		//fmt.Printf("video message: fmt %d, chunk stream id %d, message stream id %d, timestamp %d, elapsed time %d, message length %d\n", header.BasicHeader.FMT, header.BasicHeader.ChunkStreamID,
		//	header.MessageHeader.MessageStreamID, header.MessageHeader.Timestamp, header.ElapsedTime, header.MessageHeader.MessageLength)
		return m.handleVideoMessage(header.BasicHeader.ChunkStreamID, header.MessageHeader.MessageStreamID, payload, header.ElapsedTime)
	default:
		return errors.New(fmt.Sprintf("message manager: received unknown message type ID in header, ID (decimal): %d", header.MessageHeader.MessageTypeID))
	}
}

func (m *MessageManager) handleControlMessage(header *ChunkHeader, payload []byte) error {
	switch header.MessageHeader.MessageTypeID {
	case SetChunkSize:
		if config.Debug {
			fmt.Println("Received SetChunkSize control message")
		}
		// The payload of a set chunk size message is the new chunk size
		// The chunkHandler is the one affected by the chunk size, because it affects how it interprets messages.
		// ie. the chunkHandler checks to see if the message length is greater than the chunk size, if it is, it has to assemble the message from various chunks.
		newChunkSize := binary.BigEndian.Uint32(payload)
		m.session.onSetChunkSize(newChunkSize)
		return nil
	case AbortMessage:
		// The payload of an abort message is the chunk stream ID whose current message is to be discarded
		chunkStreamId := binary.BigEndian.Uint32(payload)
		m.session.onAbortMessage(chunkStreamId)
		return nil
	case Ack:
		// The payload of an ack message is the sequence number (number of bytes received so far)
		sequenceNumber := binary.BigEndian.Uint32(payload)
		m.session.onAck(sequenceNumber)
		return nil
	case WindowAckSize:
		// the ack window size is in the first 4 bytes
		windowAckSize := binary.BigEndian.Uint32(payload[:4])
		// Set the window ack size in the chunk handler, the chunk handler will call our onWindowAckSize function when the window ack size is reached
		m.session.onSetWindowAckSize(windowAckSize)
		return nil
	case SetPeerBandwidth:
		// window ack size is in the first 4 bytes: 0-3
		windowAckSize := binary.BigEndian.Uint32(payload[:4])
		// limit is the 5th byte: 4
		limitType := payload[4]

		m.session.onSetBandwidth(windowAckSize, limitType)
		return nil
	default:
		return errors.New(fmt.Sprintf("message manager: received unsupported message type ID in control message, message type ID received %d", header.MessageHeader.MessageTypeID))
	}
}

func (m *MessageManager) handleUserControlMessage(header *ChunkHeader, eventType uint16, payload []byte) error {
	switch eventType {
	case EventStreamBegin:
		m.streamID = binary.BigEndian.Uint32(payload)
		m.session.onStreamBegin()
		return nil
	default:
		fmt.Println("message manager: user control message not implemented, event type:", eventType)
		return nil
	}
}

func (m *MessageManager) handleCommandMessage(csID uint32, streamID uint32, commandType uint8, payload []byte) error {
	switch commandType {
	case CommandMessageAMF0:
		commandName, err := amf0.Decode(payload) // Decode the command name (always the first string in the payload)
		if err != nil {
			return err
		}
		m.handleCommandAmf0(csID, streamID, commandName.(string), payload[amf0.Size(commandName.(string)):])
		return nil
	case CommandMessageAMF3:
		// TODO: implement AMF3
		fmt.Println("received AMF3 command but didn't process it because AMF3 encoding/decoding is not implemented yet")
		return nil
	}
	return errors.New(fmt.Sprintf("Command is not an AMF0 nor an AMF3 command, command message received was %d", commandType))
}

func (m *MessageManager) handleCommandAmf0(csID uint32, streamID uint32, commandName string, payload []byte) {
	if config.Debug {
		fmt.Println("received command", commandName)
	}
	// Every command has a transaction ID and a command object (which can be null)
	tId, _ := amf0.Decode(payload)
	byteLength := amf0.Size(tId)
	transactionId := tId.(float64)
	// Update our payload to read the next property (commandObject)
	payload = payload[byteLength:]
	cmdObject, _ := amf0.Decode(payload)
	var commandObject map[string]interface{}
	switch cmdObject.(type) {
	case nil:
		commandObject = nil
	case map[string]interface{}:
		commandObject = cmdObject.(map[string]interface{})
	case amf0.ECMAArray:
		commandObject = cmdObject.(amf0.ECMAArray)
	}
	// Update our payload to read the next property
	byteLength = amf0.Size(cmdObject)
	payload = payload[byteLength:]

	switch commandName {
	case "connect":
		m.session.onConnect(csID, transactionId, commandObject)
	case "releaseStream":
		streamKey, _ := amf0.Decode(payload)
		m.session.onReleaseStream(csID, transactionId, commandObject, streamKey.(string))
	case "FCPublish":
		streamKey, _ := amf0.Decode(payload)
		m.session.onFCPublish(csID, transactionId, commandObject, streamKey.(string))
	case "createStream":
		m.session.onCreateStream(csID, transactionId, commandObject)
	case "publish":
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
		m.session.onPublish(transactionId, commandObject, streamKey.(string), publishingType.(string))
	case "play":
		streamKey, _ := amf0.Decode(payload)
		byteLength = amf0.Size(streamKey)
		payload = payload[byteLength:]

		// Start time in seconds
		startTime, _ := amf0.Decode(payload)
		byteLength = amf0.Size(startTime)
		payload = payload[byteLength:]

		// the spec specifies that, the next values should be duration (number), and reset (bool), but VLC doesn't send them
		m.session.onPlay(streamKey.(string), startTime.(float64))
	case "FCUnpublish":
		streamKey, _ := amf0.Decode(payload)
		m.session.onFCUnpublish(commandObject, streamKey.(string))
	case "closeStream":
		m.session.onCloseStream(csID, transactionId, commandObject)
	case "deleteStream":
		streamID, _ := amf0.Decode(payload)
		m.session.onDeleteStream(commandObject, streamID.(float64))
	case "_result":
		info, _ := amf0.Decode(payload)
		m.session.onResult(info.(map[string]interface{}))
	case "onStatus":
		info, _ := amf0.Decode(payload)
		m.session.onStatus(info.(map[string]interface{}))
	default:
		fmt.Println("message manager: received command " + commandName + ", but couldn't handle it because no implementation is defined")
	}
}

func (m *MessageManager) handleDataMessage(dataType uint8, payload []byte) error {
	switch dataType {
	case DataMessageAMF0:
		dataName, err := amf0.Decode(payload) // Decode the command name (always the first string in the payload)
		if err != nil {
			return err
		}

		return m.handleDataMessageAmf0(dataName.(string), payload[amf0.Size(dataName.(string)):])
	case DataMessageAMF3:
		// TODO: implement AMF3
		fmt.Println("message manager: received AMF3 data message, but couldn't process it because AMF3 encoding/decoding is not implemented")
		return nil
	default:
		return errors.New(fmt.Sprintf("message manager: received unknown data message type, type: %d", dataType))
	}
}

func (m *MessageManager) handleDataMessageAmf0(dataName string, payload []byte) error {
	switch dataName {
	case "@setDataFrame":
		// @setDataFrame message includes a string with value "onMetadata".
		// Ignore it for now.
		onMetadata, _ := amf0.Decode(payload)
		payload = payload[amf0.Size(onMetadata):]
		// Metadata is sent as an ECMAArray
		metadata, _ := amf0.Decode(payload)
		// Handle cases where the metadata comes as an object or as an ECMAArray
		switch metadata.(type) {
		case amf0.ECMAArray:
			m.session.onMetadata(metadata.(amf0.ECMAArray))
		case map[string]interface{}:
			m.session.onMetadata(metadata.(map[string]interface{}))
		}
		return nil
	default:
		return errors.New(fmt.Sprintf("message manager: received unknown data message with name " + dataName))
	}
}

func (m *MessageManager) handleAudioMessage(chunkStreamID uint32, messageStreamID uint32, payload []byte, timestamp uint32) error {
	//hash := make([]byte, 0)
	//sha256Hash := sha256.New()
	//sha256Hash.Reset()
	//sha256Hash.Write(payload)
	//hash = sha256Hash.Sum(hash)
	//fmt.Println("received audio, hash:", string(hash))
	// Header contains sound format, rate, size, type
	audioHeader := payload[0]
	format := audio.Format((audioHeader >> 4) & 0x0F)
	sampleRate := audio.SampleRate((audioHeader >> 2) & 0x03)
	sampleSize := audio.SampleSize((audioHeader >> 1) & 1)
	channels := audio.Channel((audioHeader) & 1)
	m.session.onAudioMessage(format, sampleRate, sampleSize, channels, payload, timestamp)
	return nil
}

func (m *MessageManager) handleVideoMessage(csID uint32, messageStreamID uint32, payload []byte, timestamp uint32) error {
	//hash := make([]byte, 0)
	//sha256Hash := sha256.New()
	//sha256Hash.Reset()
	//sha256Hash.Write(payload)
	//hash = sha256Hash.Sum(hash)
	//fmt.Println("received video, hash:", hash)
	// Header contains frame type (key frame, i-frame, etc.) and format/codec (H264, etc.)
	videoHeader := payload[0]
	frameType := video.FrameType((videoHeader >> 4) & 0x0F)
	codec := video.Codec(videoHeader & 0x0F)

	m.session.onVideoMessage(frameType, codec, payload, timestamp)
	return nil
}

func (m *MessageManager) SetChunkSize(size uint32) {
	m.chunkHandler.SetChunkSize(size)
}

func (m *MessageManager) SetWindowAckSize(size uint32) {
	m.chunkHandler.SetWindowAckSize(size)
}

func (m *MessageManager) SetBandwidth(size uint32, limitType uint8) {
	m.chunkHandler.SetBandwidth(size, limitType)
}

func (m *MessageManager) sendWindowAckSize(size uint32) {
	m.chunkHandler.sendWindowAckSize(size)
}

func (m *MessageManager) sendSetPeerBandWidth(size uint32, limitType uint8) {
	m.chunkHandler.sendSetPeerBandWidth(size, limitType)
}

func (m *MessageManager) sendBeginStream(streamId uint32) {
	m.chunkHandler.sendBeginStream(streamId)
}

func (m *MessageManager) sendSetChunkSize(size uint32) {
	m.chunkHandler.sendSetChunkSize(size)
}

func (m *MessageManager) sendConnectSuccess(csID uint32) {
	m.chunkHandler.sendConnectSuccess(csID)
}

func (m *MessageManager) sendAudio(audio []byte, timestamp uint32) {
	var header []byte
	isExtendedTimestamp := timestamp >= 0xFFFFFF
	messageLength := len(audio)
	//fmt.Println("sending audio with timestamp", timestamp)
	// Always send audio as type 0 chunk
	if isExtendedTimestamp {
		header = make([]byte, 16)
		// fmt = 0 (chunk header - type 0) and chunk stream ID = 4
		header[0] = AudioChannel

		// since we have an extended timestamp, fill the timestamp with 0xFFFFFF
		header[1] = 0xFF
		header[2] = 0xFF
		header[3] = 0xFF

		// Body size
		header[4] = byte((messageLength >> 16) & 0xFF)
		header[5] = byte((messageLength >> 8) & 0xFF)
		header[6] = byte(messageLength)

		// Type ID
		header[7] = AudioMessage

		// Extended timestamp
		binary.BigEndian.PutUint32(header[8:], timestamp)

		binary.LittleEndian.PutUint32(header[12:], 1)
	} else {
		header = make([]byte, 12)

		// fmt = 0 (chunk header - type 0) and chunk stream ID = 4 (audio)
		header[0] = AudioChannel

		// timestamp
		header[1] = byte((timestamp >> 16) & 0xFF)
		header[2] = byte((timestamp >> 8) & 0xFF)
		header[3] = byte(timestamp)

		// Body size
		header[4] = byte((messageLength >> 16) & 0xFF)
		header[5] = byte((messageLength >> 8) & 0xFF)
		header[6] = byte(messageLength)

		// Type ID
		header[7] = AudioMessage

		binary.LittleEndian.PutUint32(header[8:], 1)
	}
	//fmt.Println("audio timestamp =", timestamp)
	//fmt.Println("audio header:\n", hex.Dump(header))
	// The chunk handler will divide these into more chunks if the payload is greater than the chunk size
	m.chunkHandler.send(header, audio)
	//if err != nil {
	//	fmt.Println("error sending audio", err)
	//}
	//fmt.Println("bytes written:", n)
}

func (m *MessageManager) sendVideo(video []byte, timestamp uint32) {
	//video = append([]byte{byte(0x27), 1, 0, 0, 0x50}, video...)
	var header []byte
	isExtendedTimestamp := timestamp >= 0xFFFFFF
	messageLength := len(video)

	// Always send video as type 0 chunk for playback clients
	if isExtendedTimestamp {
		header = make([]byte, 16)
		// fmt = 0 (chunk header - type 0) and chunk stream ID = 5 (video)
		header[0] = VideoChannel

		// since we have an extended timestamp, fill the timestamp with 0xFFFFFF
		header[1] = 0xFF
		header[2] = 0xFF
		header[3] = 0xFF

		// Body size
		header[4] = byte((messageLength >> 16) & 0xFF)
		header[5] = byte((messageLength >> 8) & 0xFF)
		header[6] = byte(messageLength)

		// Type ID
		header[7] = VideoMessage

		// Extended timestamp
		binary.BigEndian.PutUint32(header[8:], timestamp)

		binary.LittleEndian.PutUint32(header[12:], 1)
	} else {
		header = make([]byte, 12)
		// fmt = 0 (chunk header - type 0) and chunk stream ID = 5 (video)
		header[0] = VideoChannel

		// timestamp
		header[1] = byte((timestamp >> 16) & 0xFF)
		header[2] = byte((timestamp >> 8) & 0xFF)
		header[3] = byte(timestamp)

		// Body size
		header[4] = byte((messageLength >> 16) & 0xFF)
		header[5] = byte((messageLength >> 8) & 0xFF)
		header[6] = byte(messageLength)

		// Type ID
		header[7] = VideoMessage

		binary.LittleEndian.PutUint32(header[8:], 1)
	}
	err := m.chunkHandler.send(header, video)
	if err != nil {
		fmt.Println("message manager received error in send:", err)
	}
	//if err != nil {
	//	fmt.Println("error sending video", err)
	//}
	//fmt.Println("bytes written:", n)
}

func (m *MessageManager) sendMetadata(metadata map[string]interface{}) {
	message := generateMetadataMessage(metadata, m.streamID)
	m.chunkHandler.send(message[:12], message[12:])
}

func (m *MessageManager) sendPlayStart(info map[string]interface{}) {
	message := generateStatusMessage(4, 1, info)
	m.chunkHandler.sendBytes(message)
}

func (m *MessageManager) sendRtmpSampleAccess(audio bool, video bool) {
	message := generateDataMessageRtmpSampleAccess(audio, video)
	m.chunkHandler.sendBytes(message)
}

func (m *MessageManager) sendStatusMessage(level string, code string, description string, optionalDetails ...string) {
	infoObject := map[string]interface{}{
		"level":       level,
		"code":        code,
		"description": description,
	}
	if len(optionalDetails) > 0 && optionalDetails[0] != "" {
		infoObject["details"] = optionalDetails[0]
	}

	message := generateStatusMessage(0, 0, infoObject)
	_, err := m.chunkHandler.sendBytes(message)
	if err != nil {
		fmt.Println("error sendingi status message:", err)
	}
}

func generateDataMessageRtmpSampleAccess(audio bool, video bool) []byte {
	message := make([]byte, 12)

	// fmt & csid
	message[0] = 5

	// timestamp
	//message[1] = 0
	//message[2] = 0
	//message[3] = 0

	// body size
	message[4] = 0
	message[5] = 0
	message[6] = 24

	// message type ID
	message[7] = DataMessageAMF0

	// stream ID
	binary.LittleEndian.PutUint32(message[8:], 1)

	rtmpSampleAccess, _ := amf0.Encode("|RtmpSampleAccess")
	message = append(message, rtmpSampleAccess...)
	audioTrue, _ := amf0.Encode(audio)
	message = append(message, audioTrue...)
	videoTrue, _ := amf0.Encode(video)
	message = append(message, videoTrue...)

	return message
}

func (m *MessageManager) sendOnFCPublish(csID uint32, transactionID float64, streamKey string) {
	message := generateOnFCPublishMessage(csID, transactionID, streamKey)
	_, err := m.chunkHandler.sendBytes(message)
	if err != nil {
		fmt.Println("error sending onFCPublish", err)
	}
}

func (m *MessageManager) sendCreateStreamResponse(csID uint32, transactionID float64, data map[string]interface{}) {
	message := generateCreateStreamResponse(csID, transactionID, data)
	m.chunkHandler.sendBytes(message)
}

func (m *MessageManager) requestConnect(info map[string]interface{}) error {
	message := generateConnectRequest(3, 1, info)
	err := m.chunkHandler.send(message[:12], message[12:])
	return err
}

func (m *MessageManager) requestCreateStream(transactionID int) {
	message := generateCreateStreamRequest(transactionID)
	m.chunkHandler.send(message[:8], message[8:])
}

func (m *MessageManager) requestPlay(streamKey string) {
	message := generatePlayRequest(streamKey, m.streamID)
	m.chunkHandler.send(message[:12], message[12:])
}
