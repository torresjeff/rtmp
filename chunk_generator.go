package rtmp

import (
	"encoding/binary"
	"github.com/torresjeff/rtmp/amf/amf0"
	"github.com/torresjeff/rtmp/config"
)

const NetConnectionSucces = "NetConnection.Connect.Success"

func generateWindowAckSizeMessage(size uint32) []byte {
	windowAckSizeMessage := make([]byte, 16)
	//---- HEADER ----//
	// fmt = 0 and csid = 2 encoded in 1 byte
	windowAckSizeMessage[0] = 2
	// timestamp (3 bytes) is set to 0 so bytes 1-3 are unmodified (they're already zero-initialized)
	//windowAckSizeMessage[1] = 0
	//windowAckSizeMessage[2] = 0
	//windowAckSizeMessage[3] = 0

	// the next 3 bytes (4-6) indicate the size of the body which is 4 bytes. So set it to 4 (the first 2 bytes 4-5 are unused because the number 4 only requires 1 byte to store)
	windowAckSizeMessage[6] = 4

	// Set the type of the message. In our case this is a Window Acknowledgement Size message (5)
	windowAckSizeMessage[7] = WindowAckSize

	// The next 4 bytes indicate the Message Stream ID. Protocol Control Messages, such as Window Acknowledgement Size, always use the message stream ID 0. So leave them at 0
	// (they're already zero initialized)
	// NetConnection is the default communication channel, which has a stream ID 0. Protocol and a few command messages, including createStream, use the default communication channel.
	//windowAckSizeMessage[8] = 0
	//windowAckSizeMessage[9] = 0
	//windowAckSizeMessage[10] = 0
	//windowAckSizeMessage[11] = 0

	//---- BODY ----//
	// Finally, store the actual window size message the client should use in the last 4 bytes
	binary.BigEndian.PutUint32(windowAckSizeMessage[12:], size)

	return windowAckSizeMessage
}

func generateSetPeerBandwidthMessage(size uint32, limitType uint8) []byte {
	setPeerBandwidthMessage := make([]byte, 17)
	//---- HEADER ----//
	// fmt = 0 and csid = 2 encoded in 1 byte.
	// Chunk Stream ID with value 2 is reserved for low-level protocol control messages and commands.
	setPeerBandwidthMessage[0] = 2
	// timestamp (3 bytes) is set to 0 so bytes 1-3 are unmodified (they're already zero-initialized)
	//setPeerBandwidthMessage[1] = 0
	//setPeerBandwidthMessage[2] = 0
	//setPeerBandwidthMessage[3] = 0

	// the next 3 bytes (4-6) indicate the size of the body which is 5 bytes (4 bytes for the window ack size, 1 byte for the limit type)
	// So set it to 5 (the first 2 bytes 4-5 are unused because the number 5 only requires 1 byte to store)
	setPeerBandwidthMessage[6] = 5

	// Set the type of the message. In our case this is a Set Peer Bandwidth message (5)
	setPeerBandwidthMessage[7] = SetPeerBandwidth

	// The next 4 bytes indicate the Message Stream ID. Protocol Control Messages, such as Window Acknowledgement Size, always use the message stream ID 0. So leave them at 0
	// (they're already zero initialized)
	// NetConnection is the default communication channel, which has a stream ID 0. Protocol and a few command messages, including createStream, use the default communication channel.
	//setPeerBandwidthMessage[8] = 0
	//setPeerBandwidthMessage[9] = 0
	//setPeerBandwidthMessage[10] = 0
	//setPeerBandwidthMessage[11] = 0

	//---- BODY ----//
	// Store the actual peer bandwidth the client should use in the next 4 bytes
	binary.BigEndian.PutUint32(setPeerBandwidthMessage[12:], size)

	// Finally, set the limit type (hard = 0, soft = 1, dynamic = 2). The spec defines each one of these as follows:
	// 0 - Hard: The peer SHOULD limit its output bandwidth to the indicated window size.
	// 1 - Soft: The peer SHOULD limit its output bandwidth to the the window indicated in this message or the limit already in effect, whichever is smaller.
	// 2 - Dynamic: If the previous Limit Type was Hard, treat this message as though it was marked Hard, otherwise ignore this message.
	setPeerBandwidthMessage[16] = limitType

	return setPeerBandwidthMessage
}

func generateStreamBeginMessage(streamId uint32) []byte {
	streamBeginMessage := make([]byte, 18)
	//---- HEADER ----//
	// fmt = 0 and csid = 2 encoded in 1 byte.
	// Chunk Stream ID with value 2 is reserved for low-level protocol control messages and commands.
	streamBeginMessage[0] = 2
	// timestamp (3 bytes) is set to 0 so bytes 1-3 are unmodified (they're already zero-initialized)
	//streamBeginMessage[1] = 0
	//streamBeginMessage[2] = 0
	//streamBeginMessage[3] = 0

	// the next 3 bytes (4-6) indicate the size of the body which is 6 bytes (2 bytes for the event type, 4 bytes for the event data)
	// So set it to 6 (the first 2 bytes 4-5 are unused because the number 6 only requires 1 byte to store)
	streamBeginMessage[6] = 6

	// Set the type of the message. In our case this is a User Control Message (4)
	streamBeginMessage[7] = UserControlMessage

	// The next 4 bytes indicate the Message Stream ID. Protocol Control Messages, such as Window Acknowledgement Size, always use the message stream ID 0. So leave them at 0.
	// (they're already zero initialized)
	// NetConnection is the default communication channel, which has a stream ID 0. Protocol and a few command messages, including createStream, use the default communication channel.
	//streamBeginMessage[8] = 0
	//streamBeginMessage[9] = 0
	//streamBeginMessage[10] = 0
	//streamBeginMessage[11] = 0

	//---- BODY ----//
	// The next two bytes specify the the event type. In our case, since this is a Stream Begin message, it has event type = 0. Leave the next two bytes at 0.
	//streamBeginMessage[12] = 0
	//streamBeginMessage[13] = 0

	// The next 4 bytes specify the event the stream ID that became functional
	binary.BigEndian.PutUint32(streamBeginMessage[14:], streamId)

	return streamBeginMessage
}

func generateSetChunkSizeMessage(chunkSize uint32) []byte {
	setChunkSizeMessage := make([]byte, 16)
	//---- HEADER ----//
	// fmt = 0 and csid = 2 encoded in 1 byte
	// Chunk Stream ID with value 2 is reserved for low-level protocol control messages and commands.
	setChunkSizeMessage[0] = 2
	// timestamp (3 bytes) is set to 0 so bytes 1-3 are unmodified (they're already zero-initialized)
	//setChunkSizeMessage[1] = 0
	//setChunkSizeMessage[2] = 0
	//setChunkSizeMessage[3] = 0

	// the next 3 bytes (4-6) indicate the inChunkSize of the body which is 4 bytes. So set it to 4 (the first 2 bytes 4-5 are unused because the number 4 only requires 1 byte to store)
	setChunkSizeMessage[6] = 4

	// Set the type of the message. In our case this is a Window Acknowledgement Size message (5)
	setChunkSizeMessage[7] = SetChunkSize

	// The next 4 bytes indicate the Message Stream ID. Protocol Control Messages, such as Window Acknowledgement Size, always use the message stream ID 0. So leave them at 0
	// (they're already zero initialized)
	// NetConnection is the default communication channel, which has a stream ID 0. Protocol and a few command messages, including createStream, use the default communication channel.
	//setChunkSizeMessage[8] = 0
	//setChunkSizeMessage[9] = 0
	//setChunkSizeMessage[10] = 0
	//setChunkSizeMessage[11] = 0

	//---- BODY ----//
	// Finally, store the actual chunk size client should use in the last 4 bytes
	binary.BigEndian.PutUint32(setChunkSizeMessage[12:], chunkSize)

	return setChunkSizeMessage
}

func generateConnectResponseSuccess(csID uint32) []byte {

	// Body of our message
	commandName, _ := amf0.Encode("_result")
	// Transaction ID is 1 for connection responses
	transactionId, _ := amf0.Encode(1)
	properties, _ := amf0.Encode(map[string]interface{}{
		"fmsVer":       config.FlashMediaServerVersion,
		"capabilities": config.Capabilities,
		"mode":         config.Mode,
	})
	information, _ := amf0.Encode(map[string]interface{}{
		"code":        NetConnectionSucces,
		"level":       "status",
		"description": "Connection accepted.",
		"data": map[string]interface{}{
			"string": "3,5,7,7009",
		},
		"objectEncoding": 0, // AMFVersion0
	})

	// Calculate the body length
	bodyLength := len(commandName) + len(transactionId) + len(properties) + len(information)

	// 12 bytes for the header
	connectResponseSuccessMessage := make([]byte, 12, 300)
	//---- HEADER ----//
	// fmt = 0 and csid = 3 encoded in 1 byte
	// why does Twitch send csId = 3? is it because it is replying to the connect() request which sent csID = 3?
	connectResponseSuccessMessage[0] = byte(csID)

	// timestamp (3 bytes) is set to 0 so bytes 1-3 are unmodified (they're already zero-initialized)
	//connectResponseSuccessMessage[1] = 0
	//connectResponseSuccessMessage[2] = 0
	//connectResponseSuccessMessage[3] = 0

	// Bytes 4-6 specify the body size.
	connectResponseSuccessMessage[4] = byte((bodyLength >> 16) & 0xFF)
	connectResponseSuccessMessage[5] = byte((bodyLength >> 8) & 0xFF)
	connectResponseSuccessMessage[6] = byte(bodyLength)

	// Set type to AMF0 command (20)
	connectResponseSuccessMessage[7] = CommandMessageAMF0

	// Next 4 bytes specify the stream ID, leave it at 0
	// NetConnection is the default communication channel, which has a stream ID 0. Protocol and a few command messages, including createStream, use the default communication channel.
	//connectResponseSuccessMessage[8] = 0
	//connectResponseSuccessMessage[9] = 0
	//connectResponseSuccessMessage[10] = 0
	//connectResponseSuccessMessage[11] = 0

	//---- BODY ----//
	// Set the body
	connectResponseSuccessMessage = append(connectResponseSuccessMessage, commandName...)
	connectResponseSuccessMessage = append(connectResponseSuccessMessage, transactionId...)
	connectResponseSuccessMessage = append(connectResponseSuccessMessage, properties...)
	connectResponseSuccessMessage = append(connectResponseSuccessMessage, information...)

	return connectResponseSuccessMessage
}

func generateOnFCPublishMessage(csID uint32, transactionID float64, streamKey string) []byte {
	onFCPublishString, _ := amf0.Encode("onFCPublish")
	tId, _ := amf0.Encode(0)
	commandObject, _ := amf0.Encode(nil)
	information, _ := amf0.Encode(map[string]interface{}{
		"level":       "status",
		"code":        "NetStream.Publish.Start",
		"description": "FCPublish to stream " + streamKey,
	})
	bodyLength := len(onFCPublishString) + len(tId) + len(commandObject) + len(information)

	onFCPublishMessage := make([]byte, 12, 180)
	//---- HEADER ----//
	// if csid = 3, does this mean these are not control messages/commands?
	onFCPublishMessage[0] = byte(csID)

	// Leave timestamp at 0 (bytes 1-3)

	// Set body size (bytes 4-6) to bodyLength
	onFCPublishMessage[4] = byte((bodyLength >> 16) & 0xFF)
	onFCPublishMessage[5] = byte((bodyLength >> 8) & 0xFF)
	onFCPublishMessage[6] = byte(bodyLength)

	// Set type to AMF0 command (20)
	onFCPublishMessage[7] = CommandMessageAMF0

	// Leave stream ID at 0 (bytes 8-11)
	// NetConnection is the default communication channel, which has a stream ID 0. Protocol and a few command messages, including createStream, use the default communication channel.

	//---- BODY ----//
	onFCPublishMessage = append(onFCPublishMessage, onFCPublishString...)
	onFCPublishMessage = append(onFCPublishMessage, tId...)
	onFCPublishMessage = append(onFCPublishMessage, commandObject...)
	onFCPublishMessage = append(onFCPublishMessage, information...)

	return onFCPublishMessage
}

func generateCreateStreamResponse(csID uint32, transactionID float64, commandObject map[string]interface{}) []byte {
	result, _ := amf0.Encode("_result")
	tID, _ := amf0.Encode(transactionID)
	commandObjectResponse, _ := amf0.Encode(nil)
	// ID of the stream that was opened. We could also send an object with additional information if an error occurred, instead of a number.
	// Subsequent chunks will be sent by the client on the stream ID specified here.
	// TODO: is this a fixed value?
	streamID, _ := amf0.Encode(config.DefaultStreamID)
	bodyLength := len(result) + len(tID) + len(commandObjectResponse) + len(streamID)

	createStreamResponseMessage := make([]byte, 12, 50)
	//---- HEADER ----//
	// if csid = 3, does this mean these are not control messages/commands?
	createStreamResponseMessage[0] = byte(csID)

	// Leave timestamp at 0 (bytes 1-3)

	// Set body size (bytes 4-6) to bodyLength
	createStreamResponseMessage[4] = byte((bodyLength >> 16) & 0xFF)
	createStreamResponseMessage[5] = byte((bodyLength >> 8) & 0xFF)
	createStreamResponseMessage[6] = byte(bodyLength)

	// Set type to AMF0 command (20)
	createStreamResponseMessage[7] = CommandMessageAMF0

	// Leave stream ID at 0 (bytes 8-11)
	// NetConnection is the default communication channel, which has a stream ID 0. Protocol and a few command messages, including createStream, use the default communication channel.

	//---- BODY ----//
	createStreamResponseMessage = append(createStreamResponseMessage, result...)
	createStreamResponseMessage = append(createStreamResponseMessage, tID...)
	createStreamResponseMessage = append(createStreamResponseMessage, commandObjectResponse...)
	createStreamResponseMessage = append(createStreamResponseMessage, streamID...)

	return createStreamResponseMessage
}

func generateConnectRequest(csID int, transactionID int, info map[string]interface{}) []byte {
	connect, _ := amf0.Encode("connect")
	tID, _ := amf0.Encode(transactionID)
	cmdObj, _ := amf0.Encode(info)
	bodyLength := len(connect) + len(tID) + len(cmdObj)

	connectRequestMessage := make([]byte, 12, 12+bodyLength)
	//---- HEADER ----//
	// if csid = 3, does this mean these are not control messages/commands?
	connectRequestMessage[0] = byte(csID)

	// Leave timestamp at 0 (bytes 1-3)

	// Set body size (bytes 4-6) to bodyLength
	connectRequestMessage[4] = byte((bodyLength >> 16) & 0xFF)
	connectRequestMessage[5] = byte((bodyLength >> 8) & 0xFF)
	connectRequestMessage[6] = byte(bodyLength)

	// Set type to AMF0 command (20)
	connectRequestMessage[7] = CommandMessageAMF0

	// Leave stream ID at 0 (bytes 8-11)
	// NetConnection is the default communication channel, which has a stream ID 0. Protocol and a few command messages, including createStream, use the default communication channel.

	//---- BODY ----//
	connectRequestMessage = append(connectRequestMessage, connect...)
	connectRequestMessage = append(connectRequestMessage, tID...)
	connectRequestMessage = append(connectRequestMessage, cmdObj...)

	return connectRequestMessage
}

func generateCreateStreamRequest(transactionID int) []byte {
	createStream, _ := amf0.Encode("createStream")
	tID, _ := amf0.Encode(transactionID)
	cmdObj, _ := amf0.Encode(nil)
	bodyLength := len(createStream) + len(tID) + len(cmdObj)
	createStreamMessage := make([]byte, 8, 8+bodyLength)

	//---- HEADER ----//
	// if csid = 3, does this mean these are not control messages/commands?
	createStreamMessage[0] = byte(3)

	// Leave timestamp delta at 0 (bytes 1-3)

	// Set body size (bytes 4-6) to bodyLength
	createStreamMessage[4] = byte((bodyLength >> 16) & 0xFF)
	createStreamMessage[5] = byte((bodyLength >> 8) & 0xFF)
	createStreamMessage[6] = byte(bodyLength)

	// Set type to AMF0 command (20)
	createStreamMessage[7] = CommandMessageAMF0

	//---- BODY ----//
	createStreamMessage = append(createStreamMessage, createStream...)
	createStreamMessage = append(createStreamMessage, tID...)
	createStreamMessage = append(createStreamMessage, cmdObj...)

	return createStreamMessage
}

func generateMetadataMessage(metadata map[string]interface{}, streamID uint32) []byte {
	setDataFrame, _ := amf0.Encode("@setDataFrame")
	onMetadata, _ := amf0.Encode("onMetadata")
	metadataObj, _ := amf0.Encode(amf0.ECMAArray(metadata))
	bodyLength := len(setDataFrame) + len(onMetadata) + len(metadataObj)
	metadataMessage := make([]byte, 12, 12+bodyLength)

	//---- HEADER ----//
	metadataMessage[0] = byte(4)

	// Leave timestamp at 0 (bytes 1-3)
	// TODO: what to do to avoid resetting media timestamp with a metadata message? This is a type 0 chunk, meaning it will reset the timestamp...

	// Set body size (bytes 4-6) to bodyLength
	metadataMessage[4] = byte((bodyLength >> 16) & 0xFF)
	metadataMessage[5] = byte((bodyLength >> 8) & 0xFF)
	metadataMessage[6] = byte(bodyLength)

	// Set type to AMF0 Data Message (18)
	metadataMessage[7] = DataMessageAMF0

	// Set stream ID
	binary.LittleEndian.PutUint32(metadataMessage[8:], streamID)

	//---- BODY ----//
	metadataMessage = append(metadataMessage, setDataFrame...)
	metadataMessage = append(metadataMessage, onMetadata...)
	metadataMessage = append(metadataMessage, metadataObj...)
	return metadataMessage
}

func generatePlayRequest(streamKey string, streamID uint32) []byte {
	play, _ := amf0.Encode("play")
	tID, _ := amf0.Encode(0)
	cmdObj, _ := amf0.Encode(nil)
	streamName, _ := amf0.Encode(streamKey)
	start, _ := amf0.Encode(-2000)
	bodyLength := len(play) + len(tID) + len(cmdObj) + len(streamName) + len(start)
	playMessage := make([]byte, 12, 12+bodyLength)

	//---- HEADER ----//
	// if csid = 3, does this mean these are not control messages/commands?
	playMessage[0] = byte(3)

	// Leave timestamp at 0 (bytes 1-3)

	// Set body size (bytes 4-6) to bodyLength
	playMessage[4] = byte((bodyLength >> 16) & 0xFF)
	playMessage[5] = byte((bodyLength >> 8) & 0xFF)
	playMessage[6] = byte(bodyLength)

	// Set type to AMF0 command (20)
	playMessage[7] = CommandMessageAMF0

	// Set stream ID
	binary.LittleEndian.PutUint32(playMessage[8:], streamID)

	//---- BODY ----//
	playMessage = append(playMessage, play...)
	playMessage = append(playMessage, tID...)
	playMessage = append(playMessage, cmdObj...)
	playMessage = append(playMessage, streamName...)
	playMessage = append(playMessage, start...)

	return playMessage
}

func generateStatusMessage(transactionID float64, streamID uint32, infoObject map[string]interface{}) []byte {

	commandName, _ := amf0.Encode("onStatus")
	tID, _ := amf0.Encode(transactionID)
	// Status messages don't have a command object, so encode nil
	commandObject, _ := amf0.Encode(nil)
	info, _ := amf0.Encode(infoObject)
	bodyLength := len(commandName) + len(tID) + len(commandObject) + len(info)

	createStreamResponseMessage := make([]byte, 12, 150)
	//---- HEADER ----//
	// if csid = 3, does this mean these are not control messages/commands?
	createStreamResponseMessage[0] = 3 // Twitch sends 3

	// Leave timestamp at 0 (bytes 1-3)

	// Set body size (bytes 4-6) to bodyLength
	createStreamResponseMessage[4] = byte((bodyLength >> 16) & 0xFF)
	createStreamResponseMessage[5] = byte((bodyLength >> 8) & 0xFF)
	createStreamResponseMessage[6] = byte(bodyLength)

	// Set type to AMF0 command (20)
	createStreamResponseMessage[7] = CommandMessageAMF0

	// Set stream ID to whatever stream ID the request had (bytes 8-11). Stream ID is stored in LITTLE ENDIAN format.
	binary.LittleEndian.PutUint32(createStreamResponseMessage[8:], streamID)

	//---- BODY ----//
	createStreamResponseMessage = append(createStreamResponseMessage, commandName...)
	createStreamResponseMessage = append(createStreamResponseMessage, tID...)
	createStreamResponseMessage = append(createStreamResponseMessage, commandObject...)
	createStreamResponseMessage = append(createStreamResponseMessage, info...)

	return createStreamResponseMessage
}

func generateAckMessage(sequenceNumber uint32) []byte {
	setChunkSizeMessage := make([]byte, 16)
	//---- HEADER ----//
	// fmt = 0 and csid = 2 encoded in 1 byte
	// Chunk Stream ID with value 2 is reserved for low-level protocol control messages and commands.
	setChunkSizeMessage[0] = 2
	// timestamp (3 bytes) is set to 0 so bytes 1-3 are unmodified (they're already zero-initialized)

	// the next 3 bytes (4-6) indicate the size of the body which is 4 bytes. So set it to 4 (the first 2 bytes 4-5 are unused because the number 4 only requires 1 byte to store)
	setChunkSizeMessage[6] = 4

	// Set the type of the message. In our case this is an Acknowledgement message (3)
	setChunkSizeMessage[7] = Ack

	// The next 4 bytes indicate the Message Stream ID. Protocol Control Messages, such as Window Acknowledgement Size, always use the message stream ID 0. So leave them at 0
	// (they're already zero initialized)

	//---- BODY ----//
	// Finally, store the actual sequence number (number of bytes received so far) in the last 4 bytes
	binary.BigEndian.PutUint32(setChunkSizeMessage[12:], sequenceNumber)

	return setChunkSizeMessage
}
