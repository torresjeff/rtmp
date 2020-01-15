package video

// As defined in the FLV spec: https://www.adobe.com/content/dam/acom/en/devnet/flv/video_file_format_spec_v10_1.pdf

type FrameType uint8
const (
	KeyFrame FrameType = 1
	InterFrame FrameType = 2
	DisposableInterFrame FrameType = 3
	GeneratedKeyFrame FrameType = 4
	// Video info/command frame
	CommandFrame FrameType = 5
)

type Codec uint8
const (
	SorensonH263    Codec = 2
	ScreenVideo     Codec = 3
	VP6             Codec = 4
	VP6AlphaChannel Codec = 5
	ScreenVideoV2   Codec = 6
	H264            Codec = 7
)

type AVCPacketType uint8
const (
	AVCSequenceHeader AVCPacketType = 0
	AVCNALU AVCPacketType = 1
	AVCEndOfSequence AVCPacketType = 2
)