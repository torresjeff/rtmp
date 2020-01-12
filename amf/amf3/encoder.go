package amf3

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// Encodes the value v into its AMF3 form.
// If you encode a signed/unsigned int greater than MaxInt, it will be encoded as a double.
// If you encode a signed int less than MinInt, it will be encoded as a double.
func Encode(v interface{}) ([]byte, error) {
	switch v.(type) {
	case nil:
		return encodeNull(), nil
	case bool:
		return encodeBool(v.(bool)), nil
	case int:
		return encodeInt(v.(int)), nil
	case uint:
		return encodeInt(int(v.(uint))), nil
	//case string:
	//	return encodeString(v.(string)), nil
	case time.Time:
		return encodeDate(v.(time.Time)), nil
	//case []interface{}: // TODO: implement
	//	return encodeArray(v.([]interface{})), nil
	//case map[string]interface{}:
	//	return encodeObject(v.(map[string]interface{})), nil
	default:
		return nil, errors.New(fmt.Sprintf("cannot encode type %T", v))
	}
}

// TODO: implement
func encodeObject(i map[string]interface{}) []byte {
	return nil
}

func encodeDate(t time.Time) []byte {
	buf := make([]byte, 0)
	buf = append(buf, TypeDate) // 1 byte header
	buf = append(buf, encodeU29(1)...) // 1 indicates explicit timestamp (ie. not a reference)

	timestamp := t.UnixNano()/1000000
	buf = append(buf, encodeDouble(float64(timestamp))...)
	// Last 2 bytes are time zone (which should stay with a value of 0 as defined by the spec)

	return buf
}

// TODO: implement
func encodeString(s string) []byte {
	if s == "" {

	}
	return nil
}

func encodeU29(i int) []byte {
	// The high bit of the first 3 bytes are used as flags to determine whether the next byte is part of the integer.
	useNextByte := 0x80 // all bits are zero except for the highest bit. This is equal to 1000 0000 in binary
	i &= 0x1FFFFFFF
	if i <= 0x7F {
		// If number is less than 128 it can be stored in one byte
		var ret [2]byte
		ret[0] = TypeInteger
		ret[1] = byte(i)
		return ret[:]
	} else if i <= 0x3FFF {
		// If number is in range 128 - 16,383 (both inclusive)
		var ret [3]byte
		ret[0] = TypeInteger
		ret[1] = byte((i >> 7) | useNextByte)
		ret[2] = byte(i & 0x7F)
		return ret[:]
	} else if i <= 0x1FFFFF {
		// If number is in range 16,384 - 2,097,151 (both inclusive)
		var ret [4]byte
		ret[0] = TypeInteger
		ret[1] = byte((i >> 14) | useNextByte)
		ret[2] = byte((i >> 7) | useNextByte)
		ret[3] = byte(i & 0x7F)
		return ret[:]
	} else {
		// If number is in range 2,097,152 - 1,073,741,823 (both inclusive)
		var ret [5]byte
		ret[0] = TypeInteger
		ret[1] = byte((i >> 22) | useNextByte)
		ret[2] = byte((i >> 15) | useNextByte)
		ret[3] = byte((i >> 8) | useNextByte)
		// a 4 byte integer, uses all 8 bits of the last byte completely
		ret[4] = byte(i)
		return ret[:]
	}
}

func encodeInt(i int) []byte {
	// AMF3 ints are variable sized. The largest UNSIGNED integer that can be represented is 2^29 - 1 (268,435,455 is the maximum SIGNED integer)

	if i >= MinInt && i <= MaxInt {
		return encodeU29(i)
	} else {
		// If the number is greater than MaxInt or less than MinInt, serialize it as a double (as per the spec)
		return encodeDouble(float64(i))
	}
}

func encodeDouble(f float64) []byte {
	var ret [9]byte
	ret[0] = TypeDouble
	binary.BigEndian.PutUint64(ret[1:], uint64(f))
	return ret[:]
}

func encodeBool(b bool) []byte {
	var buf [1]byte
	if b {
		buf [0] = TypeTrue
	} else {
		buf[0] = TypeFalse
	}
	return buf [:]
}

func encodeNull() []byte {
	var null [1]byte
	null[0] = TypeNull
	return null[:]
}