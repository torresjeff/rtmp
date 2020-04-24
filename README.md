# RTMP Server
RTMP server written in Go (Golang) that allows stream publishing.

## Install
`go get github.com/torresjeff/rtmp`

## How to start your RTMP server
Start up a server for the ingestion/playback of an RTMP stream (default port is 1935):

```
package main

import (
	"github.com/torresjeff/rtmp"
	"log"
)

func main() {
	server := &rtmp.Server{}
	log.Fatal(server.Run())
}
```

You can also create a client to listen for events on a stream (eg: audio, video, and metadata events) so you can do further processing on the media that is being streamed:
```
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
	fmt.Printf("client: on metadata: %+v", metadata) 
}

func main() {
	// Specify audio, video and metadata callbacks
	client := &rtmp.Client{
		OnAudio: OnAudio,
		OnVideo: OnVideo,
		OnMetadata: OnMetadata,
	}

	log.Fatal(client.Connect("rtmp://192.168.1.2/app/publish"))
}
```

To view other options accepted by the Server and Client structs, look at the `examples` directory.
## Additional notes
- This is a work in progress. 
