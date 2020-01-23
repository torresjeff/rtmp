package main

import (
	"fmt"
	"github.com/torresjeff/rtmp"
	"github.com/torresjeff/rtmp/audio"
	"github.com/torresjeff/rtmp/video"
	"log"
)

func OnAudio(format audio.Format, sampleRate audio.SampleRate, sampleSize audio.SampleSize, channels audio.Channel, payload []byte, timestamp uint32) {
	fmt.Println("client: on audio")
}

func OnVideo(frameType video.FrameType, codec video.Codec, payload []byte, timestamp uint32) {
	fmt.Println("client: on video")
}

func OnMetadata(metadata map[string]interface{}) {
	fmt.Println("client: on metadata")
}

func main() {
	// Specify audio, video and metadata callbacks
	client := &rtmp.Client{
		OnAudio: OnAudio,
		OnVideo: OnVideo,
		OnMetadata: OnMetadata,
	}

	log.Fatal(client.Connect("rtmp://192.168.1.2/app/publish"))
	//log.Fatal(client.Connect("rtmp://live-atl.twitch.tv/app/stremKey"))
}