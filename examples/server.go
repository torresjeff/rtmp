package main

import (
	"github.com/torresjeff/rtmp"
	"go.uber.org/zap"
)

func main() {
	//defer profile.Start(profile.CPUProfile, profile.ProfilePath(".")).Stop()
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	server := &rtmp.Server{
		Logger:      logger,
		Broadcaster: rtmp.NewBroadcaster(rtmp.NewInMemoryContext()),
	}

	logger.Fatal(server.Listen().Error())
}
