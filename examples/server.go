package main

import (
	"github.com/torresjeff/rtmp"
	"github.com/torresjeff/rtmp/config"
	"log"
)

func main() {
	//defer profile.Start(profile.CPUProfile, profile.ProfilePath(".")).Stop()
	server := &rtmp.Server{}
	//server.Addr = ":1936"
	config.Debug = true
	log.Fatal(server.Run())
}
