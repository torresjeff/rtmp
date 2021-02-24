package rtmp

type MessageType uint8

const (
	SetChunkSize MessageType = 1 + iota
	AbortMessage
	Acknowledgement
	UserControlMessage
	WindowAcknowledgementSize
	SetPeerBandwidth

	AudioMessage = 8
	VideoMessage = 9

	DataMessageAMF3         = 15
	SharedObjectMessageAMF3 = 16
	CommandMessageAMF3      = 17

	DataMessageAMF0         = 18
	SharedObjectMessageAMF0 = 19
	CommandMessageAMF0      = 20

	AggregateMessage = 22
)

// Message defines the basic structure of a message that is sent through the RTMP chunk stream
// The timestamp field can contain the 3 byte representation or the 4 byte representation (extended timestamp).
// It will be automatically encoded/decoded properly in the chunk stream.
type Message struct {
	messageType    MessageType
	length         uint32
	timestamp      uint32
	timestampDelta uint32
	streamId       uint32
	payload        []byte
}
