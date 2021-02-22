package rtmp

type ChunkType uint8

const (
	ChunkType0 ChunkType = iota
	ChunkType1
	ChunkType2
)

type Chunk struct {
	header  ChunkHeader
	payload []byte
}

// ChunkHeader contains the information used in order to interpret a chunk correctly.
// It includes the chunk type (fmt), message length, message timestamp, among other data.
type ChunkHeader struct {
	fmt               ChunkType
	csid              uint32
	messageTimestamp  uint32
	messageLength     uint32
	messageType       MessageType
	messageStreamId   uint32
	extendedTimestamp uint32
}
