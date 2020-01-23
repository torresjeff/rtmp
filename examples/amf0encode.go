package main

import (
	"encoding/hex"
	"fmt"
	"github.com/torresjeff/rtmp/amf/amf0"
	"time"
)

func formatHexString(hexString string) string {
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

type Foo struct {
}

func main() {
	obj := amf0.ECMAArray{
		"age": 4,
		"name": "Jefff",
		//"timeee": time.Now(),
		"objjj": map[string]interface{}{
				"ffff": 2321.543,
				"aaa": true,
				"obj2": map[string]interface{}{
					"time": time.Now(),
				},
		},
	}

	enc, _ := amf0.Encode(obj)

	bytes, err := amf0.Encode(obj)
	if err != nil {
		fmt.Println(err)
		return
	}
	hexString := hex.EncodeToString(bytes)
	fmt.Println(obj, "=>", formatHexString(hexString))

	if decode, err := amf0.Decode(enc); err != nil {
		fmt.Println("err", err)
	} else {
		fmt.Printf("decode (%T): %+v\n", decode, decode)
	}
}
