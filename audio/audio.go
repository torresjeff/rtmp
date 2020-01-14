package audio

// As defined in the FLV spec: https://www.adobe.com/content/dam/acom/en/devnet/flv/video_file_format_spec_v10_1.pdf

type Format uint8
const (
	LinearPCMPlatformEndian Format = 0
	ADPCM                   Format = 1
	MP3                     Format = 2
	LinearPCMLittleEndian   Format = 3
	Nellymoser16KHzMono     Format = 4
	Nellymoser8KHzMono      Format = 5
	Nellymoser              Format = 6
	G711AlawLogPCM          Format = 7
	G711MulawLogPCM         Format = 8
	AAC                     Format = 10
	Speex                   Format = 11
	MP38KHz                 Format = 14
	DeviceSpecificSound     Format = 15

)

type SampleRate uint8
const (
	Rate5p5KHz SampleRate = 0
	Rate11KHz  SampleRate = 1
	Rate22KHz  SampleRate = 2
	Rate44KHz  SampleRate = 3
)

type SampleSize uint8
const (
	Size8Bit  SampleSize = 0
	Size16Bit SampleSize = 1
)

type Channel uint8
const (
	Mono   Channel = 0
	Stereo Channel = 1
)

type AACPacketType uint8
const (
	AACSequenceHeader AACPacketType = 0
	AACRaw AACPacketType = 1
)
