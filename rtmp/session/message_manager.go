package session

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/torresjeff/rtmp-server/amf/amf0"
	"github.com/torresjeff/rtmp-server/config"
)

type MessageManager struct {
	session MediaServer
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
		fmt.Println("message manager: received audio message")
		return m.handleAudioMessage(header.BasicHeader.ChunkStreamID, header.MessageHeader.MessageStreamID, payload)
	case VideoMessage:
		fmt.Println("message manager: received video message")
		// TODO: implement
		return nil
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
	case "FCUnpublish":
		streamKey, _ := amf0.Decode(payload)
		m.session.onFCUnpublish(csID, transactionId, commandObject, streamKey.(string))
	case "deleteStream":
		streamID, _ := amf0.Decode(payload)
		m.session.onDeleteStream(csID, transactionId, commandObject, streamID.(float64))
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
		fmt.Println("received AMF3 data message, but couldn't process it because AMF3 encoding/decoding is not implement")
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

func (m *MessageManager) handleAudioMessage(chunkStreamID uint32, messageStreamID uint32, payload []byte) error {
	// TODO: implement. Forward to playback clients.
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

