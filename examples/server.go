package main

import (
	"github.com/torresjeff/rtmp-server/config"
	"github.com/torresjeff/rtmp-server/rtmp"
	"log"
)

func main() {
	server := &rtmp.Server{}
	//server.Addr = ":1936"
	config.Debug = true
	log.Fatal(server.Run())
}
