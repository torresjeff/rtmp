package amf

import (
	"errors"
	"fmt"
	"github.com/torresjeff/rtmp-server/amf/amf0"
	"github.com/torresjeff/rtmp-server/amf/amf3"
)

const AMFVersion0 uint8 = 0
const AMFVersion3 uint8 = 3

func Encode(v interface{}, version uint8) ([]byte, error) {
	switch version {
	case AMFVersion0:
		return amf0.Encode(v)
	case AMFVersion3:
		return amf3.Encode(v)
	default:
		return nil,errors.New(fmt.Sprintf("unsupported AMF version %d", version))
	}
}

func Decode(bytes []byte, version uint8) (interface{}, error) {
	switch version {
	case AMFVersion0:
		return amf0.Decode(bytes)
	case AMFVersion3:
		// TODO: implement decode for amf3
	default:
		return nil,errors.New(fmt.Sprintf("unsupported AMF version %d", version))
	}
}
