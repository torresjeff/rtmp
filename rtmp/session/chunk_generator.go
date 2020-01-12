package session

import (
	"encoding/binary"
	"github.com/torresjeff/rtmp-server/amf/amf0"
	"github.com/torresjeff/rtmp-server/config"
)

const NetConnectionSucces = "NetConnection.Connect.Success"

func generateWindowAckSizeMessage(size uint32) []byte {
	windowAckSizeMessage := make([]byte, 16)
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
	//windowAckSizeMessage[8] = 0
	//windowAckSizeMessage[9] = 0
	//windowAckSizeMessage[10] = 0
	//windowAckSizeMessage[11] = 0

	// Finally, store the actual window size message the client should use in the last 4 bytes
	binary.BigEndian.PutUint32(windowAckSizeMessage[12:], size)

	return windowAckSizeMessage
}

func generateSetPeerBandwidthMessage(size uint32, limitType uint8) []byte {
	setPeerBandwidthMessage := make([]byte, 17)

	// fmt = 0 and csid = 2 encoded in 1 byte.
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
	//setPeerBandwidthMessage[8] = 0
	//setPeerBandwidthMessage[9] = 0
	//setPeerBandwidthMessage[10] = 0
	//setPeerBandwidthMessage[11] = 0

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

	// fmt = 0 and csid = 2 encoded in 1 byte.
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
	//streamBeginMessage[8] = 0
	//streamBeginMessage[9] = 0
	//streamBeginMessage[10] = 0
	//streamBeginMessage[11] = 0

	// The next two bytes specify the the event type. In our case, since this is a Stream Begin message, it has event type = 0. Leave the next two bytes at 0.
	//streamBeginMessage[12] = 0
	//streamBeginMessage[13] = 0

	// The next 4 bytes specify the event the stream ID that became functional
	binary.BigEndian.PutUint32(streamBeginMessage[14:], streamId)

	return streamBeginMessage
}

func generateSetChunkSizeMessage(chunkSize uint32) []byte {
	setChunkSizeMessage := make([]byte, 16)
	// fmt = 0 and csid = 2 encoded in 1 byte
	setChunkSizeMessage[0] = 2
	// timestamp (3 bytes) is set to 0 so bytes 1-3 are unmodified (they're already zero-initialized)
	//setChunkSizeMessage[1] = 0
	//setChunkSizeMessage[2] = 0
	//setChunkSizeMessage[3] = 0

	// the next 3 bytes (4-6) indicate the chunkSize of the body which is 4 bytes. So set it to 4 (the first 2 bytes 4-5 are unused because the number 4 only requires 1 byte to store)
	setChunkSizeMessage[6] = 4

	// Set the type of the message. In our case this is a Window Acknowledgement Size message (5)
	setChunkSizeMessage[7] = SetChunkSize

	// The next 4 bytes indicate the Message Stream ID. Protocol Control Messages, such as Window Acknowledgement Size, always use the message stream ID 0. So leave them at 0
	// (they're already zero initialized)
	//setChunkSizeMessage[8] = 0
	//setChunkSizeMessage[9] = 0
	//setChunkSizeMessage[10] = 0
	//setChunkSizeMessage[11] = 0

	// Finally, store the actual chunk size client should use in the last 4 bytes
	binary.BigEndian.PutUint32(setChunkSizeMessage[12:], chunkSize)

	return setChunkSizeMessage
}

func generateConnectResponseSuccess() []byte {

	// Body of our message
	commandName, _ := amf0.Encode("_result")
	// Transaction ID is 1 for connection responses
	transactionId, _ := amf0.Encode(1)
	properties, _ := amf0.Encode(map[string]interface{}{
		"fmsVer": config.FlashMediaServerVersion,
		"capabilities": config.Capabilities,
		"mode": config.Mode,
	})
	information, _ := amf0.Encode(map[string]interface{}{
		"code": NetConnectionSucces,
		"level": "status",
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
	// fmt = 0 and csid = 3 encoded in 1 byte
	// TODO: why does Twitch send csId = 3?
	connectResponseSuccessMessage[0] = 3

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

	// Next 3 bytes specify the stream ID, leave it at 0
	//connectResponseSuccessMessage[8] = 0
	//connectResponseSuccessMessage[9] = 0
	//connectResponseSuccessMessage[10] = 0

	// Set the body
	connectResponseSuccessMessage = append(connectResponseSuccessMessage, commandName...)
	connectResponseSuccessMessage = append(connectResponseSuccessMessage, transactionId...)
	connectResponseSuccessMessage = append(connectResponseSuccessMessage, properties...)
	connectResponseSuccessMessage = append(connectResponseSuccessMessage, information...)


	return connectResponseSuccessMessage

}