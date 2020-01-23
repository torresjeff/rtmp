package main

import (
	"encoding/hex"
	"fmt"
	"github.com/torresjeff/rtmp/amf/amf3"
	"time"
)

func formatHexString2(hexString string) string {
	formattedString := "["
	for i, v := range hexString {
		if i != 0 && i%2 == 0 {
			formattedString += " "
		}
		formattedString += string(v)
	}
	formattedString += "]"
	return formattedString
}

func main() {
	//obj := map[string]interface{
	//	"age": 4,
	//	"name": "Jefff",
	//	//"timeee": time.Now(),
	//	"objjj": map[string]interface{}{
	//		"ffff": 2321.543,
	//		"aaa": true,
	//		"obj2": amf3.ECMAArray{
	//			"time": time.Now(),
	//		},
	//	},
	//}
	i := time.Now()
	bytes, err := amf3.Encode(i)
	if err != nil {
		fmt.Println(err)
		return
	}
	hexString := hex.EncodeToString(bytes)
	fmt.Println(i, "=>", formatHexString2(hexString))

	//if decode, err := amf3.Decode(enc); err != nil {
	//	fmt.Println("err", err)
	//} else {
	//	fmt.Printf("decode (%T): %+v\n", decode, decode)
	//}
}
