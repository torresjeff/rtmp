package amf3

//func Decode(bytes []byte) (interface{}, error) {
//	switch bytes[0] {
//	case TypeInteger:
//		return decodeInteger(bytes[1:]), nil
//	default:
//		return nil, errors.New(fmt.Sprintf("cannot decode type with header 0x%v (unsupported type)", hex.EncodeToString(bytes[0:1])))
//	}
//}

//func decodeInteger(bytes []byte) int32 {
//	var i int32
//	var buf [4]byte
//	useNext := true
//	useNextMask := 0x80
//	count := 0
//	for useNext {
//		buf[count] :=
//		if int(bytes[i]) & useNextMask == 0 {
//			useNext = false
//		}
//		count++
//	}
//
//	return i
//}