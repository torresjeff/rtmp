package rtmp

type MessageType uint8

const (
	SetChunkSize MessageType = 1 + iota
	AbortMessage
	Acknowledgement
	UserControlMessage
	WindowAcknowledgementSize
	SetPeerBandwidth
)
