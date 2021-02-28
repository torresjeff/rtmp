package rtmp

type ChunkType uint8

const (
	ChunkType0 ChunkType = iota
	ChunkType1
	ChunkType2
	ChunkType3
)

const (
	chunkType0MessageHeaderLength = 11
	chunkType1MessageHeaderLength = 7
	chunkType2MessageHeaderLength = 3
)

type Chunk struct {
	header  ChunkHeader
	payload []byte
}

// ChunkHeader contains the information used in order to interpret a chunk correctly.
// It includes the chunk type (chunkType), message length, message timestamp, among other data.
type ChunkHeader struct {
	chunkType            ChunkType
	chunkStreamID        uint32
	messageTimestamp     uint32
	messageLength        uint32
	messageType          MessageType
	messageStreamId      uint32
	hasExtendedTimestamp bool
	extendedTimestamp    uint32
}
