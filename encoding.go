package rtmp

type RTMPMessage interface {
	RTMPMessageMarshaler
	RTMPMessageUnmarshaler
}

type RTMPMessageMarshaler interface {
	MarshalRTMPMessage() (message []byte, err error)
}

type RTMPMessageUnmarshaler interface {
	UnmarshalRTMPMessage(message []byte) error
}
