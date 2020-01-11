package main

import (
	"encoding/hex"
	"fmt"
	"github.com/torresjeff/rtmp-server/amf/amf0"
)

func formatHexString(hexString string) string {
	formattedString := "["
	for i, v := range hexString {
		if i != 0 && i % 2 == 0 {
			formattedString += " "
		}
		formattedString += string(v)
	}
	formattedString += "]"
	return formattedString
}

func main() {
	obj := amf0.ECMAArray{
		//"timeee": time.Now(),
		//"name": "Jefff",
		"age": 4,
		//"objjj": map[string]interface{}{
		//	"ffff": 2321.543,
		//	"aaa": true,
		//},

	}

	bytes, err := amf0.Encode(obj)
	if err != nil {
		fmt.Println(err)
		return
	}
	hexString := hex.EncodeToString(bytes)
	fmt.Println(obj, "=>", formatHexString(hexString))
}