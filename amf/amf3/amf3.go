package amf3

const MaxInt int = 268435455
const MinInt int = -268435456

const UTF8Empty byte = 0x01


const (
	TypeUndefined byte = 0x00
	TypeNull = 0x01
	TypeFalse = 0x02
	TypeTrue = 0x03
	TypeInteger = 0x04
	TypeDouble = 0x05
	TypeString = 0x06
	TypeXmlDoc = 0x07
	TypeDate = 0x08
	TypeArray = 0x09
	TypeObject =0x0A
	TypeXml = 0x0B
	TypeByteArray = 0x0C
	TypeVectorInt = 0x0D
	TypeVectorUint = 0x0E
	TypeVectorDouble = 0x0F
	TypeVectorObject = 0x10
	TypeDictionary = 0x11
)