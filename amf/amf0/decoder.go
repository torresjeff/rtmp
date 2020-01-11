package amf0

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// Decode returns the original form of the encoded value.
// Possible return types: float64, bool, string, map[string]interface{}, nil, amf0.ECMAArray, time.Time
// If the contents of b represent a Number (either int or float), it will be returned as a float64
func Decode(bytes []byte) (interface{}, error) {
	// End of object
	if bytes[0] == 0x00 && bytes[1] == 0x00 && bytes[2] == TypeObjectEnd {
		return ObjectEnd{}, nil
	}
	switch bytes[0] {
	case TypeNumber:
		return decodeNumber(bytes[1:]), nil
	case TypeBoolean:
		return decodeBoolean(bytes[1]), nil
	case TypeString:
		length := uint32(binary.BigEndian.Uint16(bytes[1:3]))
		return decodeString(bytes[3:], length), nil
	case TypeLongString:
		length := binary.BigEndian.Uint32(bytes[1:5])
		return decodeString(bytes[5:], length), nil
	case TypeObject:
		return decodeObject(bytes[1:]), nil
	case TypeNull:
		return nil, nil
	case TypeECMAArray:
		return decodeECMAArray(bytes[1:]), nil
	case TypeDate:
		return decodeDate(bytes[1:]), nil
	default:
		return nil, errors.New(fmt.Sprintf("cannot decode type with header 0x%v (unsupported type)", hex.EncodeToString(bytes[0:1])))
	}
}

func decodeECMAArray(bytes []byte) ECMAArray {
	ret := make(ECMAArray)
	// Number of properties the object has
	associativeCount := binary.BigEndian.Uint32(bytes[:4])
	bytes = bytes[4:]
	for i := uint32(0); i < associativeCount; i++ {
		// Decode the key
		key, _ := Decode(bytes)
		// Update slice to start at next value
		bytes = bytes[size(key):]

		val, _ := Decode(bytes)
		ret[key.(string)] = val
		// Update slice to start at next key
		bytes = bytes[size(val):]
	}
	return ret
}

// size returns the number of bytes the value v has in its AMF0 representation.
// Eg: a value v of "test" will return 7 (3 bytes for the header, 4 bytes for the string size)
// Eg: a value v of 5 will return 9 (1 byte for the header, 8 bytes for the number)
func size(v interface{}) uint64 {
	switch v.(type) {
	case float64:
		// A float64 spans 9 bytes (1 header byte + 8 data bytes)
		return 9
	case bool:
		// A bool spans 2 bytes (1 header byte + 1 data byte)
		return 2
	case string:
		// A string's size is variable. Depends if it is a Long String (strings with length >= 65535) or a normal String (length < 65535) + its data.
		// First check for normal string
		length := uint64(len(v.(string)))
		if length < 65535 {
			// If it is a normal string, its size is 3 + n (3 header bytes + string size)
			return 3 + length
		} else {
			// If it is a long string, its size is 5 + n (5 header bytes + string size)
			return 5 + length
		}
	case map[string]interface{}:
		// Calculate object size recursively
		var objSize uint64
		for k, val := range v.(map[string]interface{}) {
			objSize += size(k)
			objSize += size(val)
		}
		return objSize + 4 // Objects have a header of 1 byte and trailing marker of 3 bytes (0x00 0x00 0x09)
	case nil:
		// nil/null has a size of 1
		return 1
	case ECMAArray:
		var objSize uint64
		for k, val := range v.(ECMAArray) {
			objSize += size(k)
			objSize += size(val)
		}
		return objSize + 5 // Objects have a header of 5 bytes (1 byte to indicate ECMArray type, followed by 4 bytes for the associative count)
	case time.Time:
		// Dates have 11 bytes
		return 11
	default:
		return 0
	}
}

func decodeObject(bytes []byte) map[string]interface{} {
	m := make(map[string]interface{})

	// Decode until an end of object is reached
	for {
		// Decode key
		key, _ := Decode(bytes)
		// Break out of the loop when we reach the end of an object
		if _, isEndOfObject := key.(ObjectEnd); isEndOfObject {
			break
		}
		// Update slice to start at next value
		bytes = bytes[size(key):]
		// Decode value
		val, _ := Decode(bytes)
		m[key.(string)] = val
		// Update our slice to start at the next key
		bytes = bytes[size(val):]
	}
	return m
}

func decodeDate(bytes []byte) time.Time {
	milliseconds := int64(binary.BigEndian.Uint64(bytes))
	return time.Unix(0, milliseconds * 1000000)
}

func decodeString(bytes []byte, length uint32) string {
	var sb strings.Builder
	sb.Write(bytes[:length])
	return sb.String()
}

func decodeBoolean(b byte) bool {
	return b != 0
}

func decodeNumber(bytes []byte) float64 {
	var f float64
	f = math.Float64frombits(binary.BigEndian.Uint64(bytes))
	return f
}