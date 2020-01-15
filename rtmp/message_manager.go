package rtmp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/torresjeff/rtmp-server/amf/amf0"
	"github.com/torresjeff/rtmp-server/audio"
	"github.com/torresjeff/rtmp-server/config"
	"github.com/torresjeff/rtmp-server/video"
)

type MessageManager struct {
	session      MediaServer
	chunkHandler *ChunkHandler
}

func NewMessageManager(session MediaServer, chunkHandler *ChunkHandler) *MessageManager {
	return &MessageManager{
		session: session,
		chunkHandler: chunkHandler,
	}
}

// Reads the next chunk header + data
func (m *MessageManager) nextMessage() error {
	// TODO: every time a chunk is read, update the number of read bytes
	var err error
	chunkHeader, err := m.chunkHandler.ReadChunkHeader()
	if err != nil {
		return err
	}

	payload, err := m.chunkHandler.ReadChunkData(chunkHeader)
	if err != nil {
		return err
	}

	return m.interpretMessage(chunkHeader, payload)
}

func (m *MessageManager) interpretMessage(header *ChunkHeader, payload []byte) error {
	// calculate timestamp from headers. Useful for audio/video messages
	// Header has an extended timestamp
	var timestamp uint32
	if header.MessageHeader.Timestamp == 0xFFFFFF || header.MessageHeader.TimestampDelta == 0xFFFFFF {
		timestamp = header.ExtendedTimestamp
	} else if header.BasicHeader.FMT == ChunkType0 {
		timestamp = header.MessageHeader.Timestamp
	} else {
		timestamp = header.MessageHeader.TimestampDelta
	}
	switch header.MessageHeader.MessageTypeID {
	case SetChunkSize, AbortMessage, Ack, WindowAckSize, SetPeerBandwidth:
		return m.handleControlMessage(header, payload)
	case UserControlMessage:
		return m.handleUserControlMessage(header, payload)
	case CommandMessageAMF0, CommandMessageAMF3:
		return m.handleCommandMessage(header.BasicHeader.ChunkStreamID, header.MessageHeader.MessageStreamID, header.MessageHeader.MessageTypeID, payload)
	case DataMessageAMF0, DataMessageAMF3:
		return m.handleDataMessage(header.MessageHeader.MessageTypeID, payload)
	case AudioMessage:
		return m.handleAudioMessage(header.BasicHeader.ChunkStreamID, header.MessageHeader.MessageStreamID, payload, timestamp, header.BasicHeader.FMT)
	case VideoMessage:
		return m.handleVideoMessage(header.BasicHeader.ChunkStreamID, header.MessageHeader.MessageStreamID, payload, timestamp, header.BasicHeader.FMT)
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
		// window ack size is in the first 4 bytes
		windowAckSize := binary.BigEndian.Uint32(payload[:4])
		// limit is the 5th byte
		limitType := payload[5]

		m.session.onSetBandwidth(windowAckSize, limitType)
		return nil
	default:
		return errors.New(fmt.Sprintf("message manager: received unsupported message type ID in control message, message type ID received %d", header.MessageHeader.MessageTypeID))
	}
}

func (m *MessageManager) handleUserControlMessage(header *ChunkHeader, payload []byte) error {
	// TODO: implement control messages
	return nil
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
		m.session.onPublish(csID, streamID, transactionId, commandObject, streamKey.(string), publishingType.(string))
	case "play":
		streamKey, _ := amf0.Decode(payload)
		byteLength = amf0.Size(streamKey)
		payload = payload[byteLength:]

		// Start time in seconds
		startTime, _ := amf0.Decode(payload)
		byteLength = amf0.Size(streamKey)
		payload = payload[byteLength:]

		// TODO: the spec specifies that, the next values should be duration (number), and reset (bool), but VLC doesn't send them
		m.session.onPlay(streamKey.(string), startTime.(float64))
	case "FCUnpublish":
		streamKey, _ := amf0.Decode(payload)
		m.session.onFCUnpublish(csID, transactionId, commandObject, streamKey.(string))
	case "closeStream":
		m.session.onCloseStream(csID, transactionId, commandObject)
	case "deleteStream":
		streamID, _ := amf0.Decode(payload)
		m.session.onDeleteStream(csID, transactionId, commandObject, streamID.(float64))
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
	default:
		return errors.New(fmt.Sprintf("message manager: received unknown data message type, type: %d", dataType))
	}
	return nil
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
			m.session.onSetDataFrame(metadata.(amf0.ECMAArray))
		case map[string]interface{}:
			m.session.onSetDataFrame(metadata.(map[string]interface{}))
		}
		return nil
	default:
		return errors.New(fmt.Sprintf("message manager: received unknown data message with name " + dataName))
	}
}

func (m *MessageManager) handleAudioMessage(chunkStreamID uint32, messageStreamID uint32, payload []byte, timestamp uint32, chunkType uint8) error {
	// Header contains sound format, rate, size, type
	audioHeader := payload[0]
	format := audio.Format((audioHeader >> 4) & 0x3F)
	sampleRate := audio.SampleRate((audioHeader >> 2) & 0x03)
	sampleSize := audio.SampleSize((audioHeader >> 1) & 1)
	channels := audio.Channel((audioHeader) & 1)
	m.session.onAudioMessage(format, sampleRate, sampleSize, channels, payload, timestamp, chunkType)
	return nil
}

func (m *MessageManager) handleVideoMessage(csID uint32, messageStreamID uint32, payload []byte, timestamp uint32, chunkType uint8) error {
	// Header contains frame type (key frame, i-frame, etc.) and format/codec (H264, etc.)
	videoHeader := payload[0]
	frameType := video.FrameType((videoHeader >> 4) & 0x0F)
	codec := video.Codec(videoHeader & 0x0F)
	m.session.onVideoMessage(frameType, codec, payload, timestamp, chunkType)
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

func (m *MessageManager) sendAudio(audio []byte, timestamp uint32, chunkType uint8) {
	var header []byte
	extendedTimestamp := timestamp >= 0xFFFFFF
	messageLength := len(audio)
	// TODO: check if this is the first time sending audio to the client, if it is, we need to send a type 0 chunk, followed by type 1 chunks
	// TODO: with this check, we could decouple chunkType
	if chunkType == ChunkType0 {
		if extendedTimestamp {
			header = make([]byte, 16)
			// TODO: determine the chunk stream at run time
			// TODO: Should session store the chunk stream ID that it's using to communicate?
			// TODO: handle cases where csid is greater than 1 byte
			// fmt = 0 (chunk header - type 0) and chunk stream ID = 4
			header[0] = 4

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

			// TODO: determine stream ID at runtime, we should store the stream ID that was created with the createStream command
			binary.LittleEndian.PutUint32(header[12:], 1)
		} else {
			header = make([]byte, 12)

			// TODO: determine the chunk stream at run time
			// TODO: Should session store the chunk stream ID that it's using to communicate?
			// TODO: handle cases where csid is greater than 1 byte
			// fmt = 0 (chunk header - type 0) and chunk stream ID = 4
			header[0] = 4

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

			// TODO: determine stream ID at runtime, we should store the stream ID that was created with the createStream command
			binary.LittleEndian.PutUint32(header[8:], 1)
		}
	} else if chunkType == ChunkType1 {
		if extendedTimestamp {
			header = make([]byte, 12)

			// TODO: determine the chunk stream at run time
			// TODO: Should session store the chunk stream ID that it's using to communicate?
			// TODO: handle cases where csid is greater than 1 byte
			// fmt = 1 (chunk header - type 1) and chunk stream ID = 4
			header[0] = (1 << 6) | 4

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

			// This is a type 1 chunk header, so no need to specify stream ID

		} else {
			header = make([]byte, 8)

			// TODO: determine the chunk stream at run time
			// TODO: Should session store the chunk stream ID that it's using to communicate?
			// TODO: handle cases where csid is greater than 1 byte
			// fmt = 1 (chunk header - type 1) and chunk stream ID = 4
			header[0] = (1 << 6) | 4

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

			// This is a type 1 chunk header, so no need to specify stream ID
		}
	} else {
		// TODO: implement chunk type 2 and type 3
	}

	// The chunk handler will divide these into more chunks if the payload is greater than the chunk size
	m.chunkHandler.send(header, audio)
}

func (m *MessageManager) sendVideo(video []byte, timestamp uint32, chunkType uint8) {
	//m.chunkHandler.send(video)
}



