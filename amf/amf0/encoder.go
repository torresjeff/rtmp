package amf0

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"
)

func Encode(v interface{}) ([]byte, error) {
	switch v.(type) {
	case float64:
		return encodeNumber(v.(float64)), nil
	case int:
		return encodeNumber(float64(v.(int))), nil
	case bool:
		return encodeBoolean(v.(bool)), nil
	case string:
		return encodeString(v.(string)), nil
	case map[string]interface{}:
		return encodeObject(v.(map[string]interface{})), nil
	case nil:
		return encodeNull(), nil
	case ECMAArray:
		return encodeECMAArray(v.(ECMAArray)), nil
	case time.Time:
		return encodeDate(v.(time.Time)), nil
	default:
		return nil, errors.New(fmt.Sprintf("cannot encode type %T", v))
	}
}

func encodeDate(t time.Time) []byte {
	timestamp := t.UnixNano()/1000000
	var buf [11]byte
	buf[0] = TypeDate
	binary.BigEndian.PutUint64(buf[1:], uint64(timestamp))
	// Last 2 bytes are time zone (which should stay with a value of 0 as defined by the spec)

	return buf[:]
}

func encodeECMAArray(ecmaArray ECMAArray) []byte {
	obj := encodeObject(ecmaArray)
	// The actual payload of the object is the length of the object buffer, minus the header byte (1 byte) and the endObject bytes (3 bytes).
	objPayloadLength := len(obj) - 4
	// An ECMA Array is an object that has additional information (associative count - 4 bytes, this is the number of keys)
	buf := make([]byte, 1 + 4 + objPayloadLength)
	buf[0] = TypeECMAArray
	// Put the associative count (how many keys the object has)
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(ecmaArray)))
	// Copy the object's payload (starts at byte 1 to ignore the header of the object
	copy(buf[5:], obj[1:1+objPayloadLength])
	return buf
}

func encodeNull() []byte {
	var buf [1]byte
	buf[0] = TypeNull
	return buf[:]
}

func encodeObject(m map[string]interface{}) []byte {
	buf := &bytes.Buffer{}
	for key := range m {
		// Encode property name
		prop, _ := Encode(key)
		// keys should not encode the type (ie. the TypeString header), it is assumed that keys are always normal strings (len(string) < 65535)
		buf.Write(prop[1:])
		// Encode property value
		val, _ := Encode(m[key])
		buf.Write(val)
	}

	buf.Write(encodeObjectEnd())
	obj := make([]byte, 1 + buf.Len())
	obj[0] = TypeObject
	copy(obj[1:], buf.Bytes())
	return obj
}

func encodeObjectEnd() []byte {
	return append([]byte{0x00, 0x00}, TypeObjectEnd)
}

func encodeString(s string) []byte {
	if len(s) < 65535 {
		// byte 0 => string type (TypeString)
		// bytes 1-2 => string length
		// bytes 3-end => string content
		str := make([]byte, 3 + len(s))
		str[0] = TypeString
		binary.BigEndian.PutUint16(str[1:3], uint16(len(s)))
		copy(str[3:], s)
		return str
	} else {
		// Strings that require more than 65535 bytes should use TypeLongString
		// byte 0 => string type (TypeLongString)
		// bytes 1-4 => string length
		// bytes 5-end => string content
		str := make([]byte, 5 + len(s))
		str[0] = TypeLongString
		binary.BigEndian.PutUint32(str[1:5], uint32(len(s)))
		copy(str[5:], s)
		return str
	}
}

func encodeBoolean(b bool) []byte {
	var buf [2]byte
	buf[0] = TypeBoolean
	if b {
		buf[1] = 1
	} else {
		buf[1] = 0
	}
	return buf[:]
}

func encodeNumber(number float64) []byte {
	var buf [9]byte
	buf[0] = TypeNumber
	binary.BigEndian.PutUint64(buf[1:], math.Float64bits(number))
	return buf[:]
}