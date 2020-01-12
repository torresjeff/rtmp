package amf0

type ECMAArray map[string]interface{}
type ObjectEnd struct{}

const (
	TypeNumber byte = 0x00
	TypeBoolean = 0x01
	TypeString = 0x02
	TypeObject = 0x03
	TypeMovieClip = 0x04 // reserved, not supported
	TypeNull = 0x05
	TypeUndefined = 0x06
	TypeReference = 0x07
	TypeECMAArray = 0x08
	TypeObjectEnd = 0x09
	TypeStrictArray = 0x0A
	TypeDate = 0x0B
	TypeLongString = 0x0C
	TypeUnsupported = 0x0D
	TypeRecordSet = 0x0E // reserved, not supported
	TypeXMLDocument = 0x0F
	TypeTypedObject = 0x10
)
