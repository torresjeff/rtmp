package main

import (
	"github.com/torresjeff/rtmp-server/config"
	"github.com/torresjeff/rtmp-server/rtmp"
	"log"
)

func main() {
	//defer profile.Start(profile.CPUProfile, profile.ProfilePath(".")).Stop()
	server := &rtmp.Server{}
	//server.Addr = ":1936"
	config.Debug = true
	log.Fatal(server.Run())
}
