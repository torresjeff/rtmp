package rtmp

import (
	"fmt"
	"github.com/torresjeff/rtmp/audio"
	"github.com/torresjeff/rtmp/config"
	"github.com/torresjeff/rtmp/rand"
	"github.com/torresjeff/rtmp/video"
	"go.uber.org/zap"
)

type AudioCallback func(format audio.Format, sampleRate audio.SampleRate, sampleSize audio.SampleSize, channels audio.Channel, payload []byte, timestamp uint32)
type VideoCallback func(frameType video.FrameType, codec video.Codec, payload []byte, timestamp uint32)
type MetadataCallback func(metadata map[string]interface{})

type surroundSound struct {
	stereoSound        bool
	twoPointOneSound   bool
	threePointOneSound bool
	fourPointZeroSound bool
	fourPointOneSound  bool
	fivePointOneSound  bool
	sevenPointOneSound bool
}

type clientMetadata struct {
	duration     float64
	fileSize     float64
	width        float64
	height       float64
	videoCodecID string
	// number representation of videoCodecID (ffmpeg sends audioCodecID as a number rather than a string (like obs))
	nVideoCodecID float64
	videoDataRate float64
	frameRate     float64
	audioCodecID  string
	// number representation of audioCodecID (ffmpeg sends audioCodecID as a number rather than a string (like obs))
	nAudioCodecID   float64
	audioDataRate   float64
	audioSampleRate float64
	audioSampleSize float64
	audioChannels   float64
	sound           surroundSound
	encoder         string
}

// Media Server interface defines the callbacks that are called when a message is received by the server
type MediaServer interface {
	// Server callbacks
	onSetChunkSize(size uint32)
	onAbortMessage(chunkStreamId uint32)
	onAck(sequenceNumber uint32)
	onSetWindowAckSize(windowAckSize uint32)
	onSetBandwidth(windowAckSize uint32, limitType uint8)
	onConnect(csID uint32, transactionId float64, data map[string]interface{})
	onReleaseStream(csID uint32, transactionId float64, args map[string]interface{}, streamKey string)
	onFCPublish(csID uint32, transactionId float64, args map[string]interface{}, streamKey string)
	onCreateStream(csID uint32, transactionId float64, data map[string]interface{})
	onPublish(transactionId float64, args map[string]interface{}, streamKey string, publishingType string)
	onFCUnpublish(args map[string]interface{}, streamKey string)
	onDeleteStream(args map[string]interface{}, streamID float64)
	onCloseStream(csID uint32, transactionId float64, args map[string]interface{})
	onAudioMessage(format audio.Format, sampleRate audio.SampleRate, sampleSize audio.SampleSize, channels audio.Channel, payload []byte, timestamp uint32)
	onVideoMessage(frameType video.FrameType, codec video.Codec, payload []byte, timestamp uint32)
	onMetadata(metadata map[string]interface{})
	onPlay(streamKey string, startTime float64)

	// TODO: separate into two distinct interfaces: client, server (maybe 3 for common functions like onAck, onSetWindowAckSize, onSetChunkSize, etc.)
	// Client callbacks
	onResult(info map[string]interface{})
	onStatus(info map[string]interface{})
	onStreamBegin()
}

// Represents a connection made with the RTMP server where messages are exchanged between client/server.
type Session struct {
	MediaServer
	logger         *zap.Logger
	sessionID      string
	clientMetadata clientMetadata
	broadcaster    *Broadcaster // broadcasts audio/video messages to playback clients subscribed to a stream
	active         bool         // true if the session is active

	// Callbacks (for RTMP clients)
	OnAudio    AudioCallback
	OnVideo    VideoCallback
	OnMetadata MetadataCallback

	// Interprets messages, calling the appropriate callback on the session. Also in charge of sending messages.
	messageManager *MessageManager

	// app data
	app            string
	flashVer       string
	swfUrl         string
	tcUrl          string
	typeOfStream   string
	streamKey      string // used to identify user
	publishingType string
	isPublisher    bool
	isPlayer       bool
	isClient       bool
	serverAddress  string
}

func NewSession(logger *zap.Logger, b *Broadcaster) *Session {
	session := &Session{
		logger:      logger,
		sessionID:   rand.GenerateUuid(),
		broadcaster: b,
		active:      true,
		isClient:    false,
	}

	return session
}

func NewClientSession(app string, tcUrl string, streamKey string, audioCallback AudioCallback, videoCallback VideoCallback, metadataCallback MetadataCallback) *Session {
	session := &Session{
		sessionID:  rand.GenerateUuid(),
		isClient:   true,
		app:        app,
		tcUrl:      tcUrl,
		streamKey:  streamKey,
		OnAudio:    audioCallback,
		OnVideo:    videoCallback,
		OnMetadata: metadataCallback,
		active:     true,
	}
	return session
}

// Start performs the initial handshake and starts receiving streams of data. This is used for servers only. For clients, use StartPlayback().
func (session *Session) Start() error {
	// Perform handshake
	err := session.messageManager.Initialize()
	if err != nil {
		return err
	}

	defer func() {
		// Remove the session from the context
		if session.isPlayer {
			if config.Debug {
				fmt.Println("session: destroying subscriber")
			}
			session.broadcaster.DestroySubscriber(session.streamKey, session.sessionID)
		}
		if session.isPublisher {
			if config.Debug {
				fmt.Println("session: destroying publisher")
			}
			// Broadcast end of stream
			session.broadcaster.BroadcastEndOfStream(session.streamKey)
			session.broadcaster.DestroyPublisher(session.streamKey)
		}
	}()

	if config.Debug {
		fmt.Println("Handshake completed successfully")
	}

	// After handshake, start reading chunks
	for {
		if session.active {
			if err = session.messageManager.nextMessage(); err != nil {
				return err
			}
		} else {
			return nil
		}
	}
}

func (session *Session) StartPlayback() error {
	err := session.messageManager.InitializeClient()

	if err != nil {
		return err
	}
	if config.Debug {
		fmt.Println("client handshake completed successfully")
	}

	info := map[string]interface{}{
		"app":           session.app,
		"flashVer":      "LNX 9,0,124,2",
		"tcUrl":         session.tcUrl,
		"fpad":          false,
		"capabilities":  15,
		"audioCodecs":   4071,
		"videoCodecs":   252,
		"videoFunction": 1,
	}

	//n, err := session.socketw.Write([]byte("Hello, world!"))
	//if err != nil {
	//	fmt.Println("error writing hello, world!", err)
	//}
	//err = session.socketw.Flush()
	//if err != nil {
	//	fmt.Println("error flushing hello, world!", err)
	//}
	//fmt.Println("bytes written:", n)

	fmt.Println("now requesting connect")
	session.messageManager.sendSetChunkSize(config.DefaultChunkSize)
	// After handshake, request connection to an application
	err = session.messageManager.requestConnect(info)
	if err != nil {
		return err
	}

	// Start reading chunks
	for {
		if session.active {
			if err = session.messageManager.nextMessage(); err != nil {
				return err
			}
		} else {
			return nil
		}
	}
}

func (session *Session) onResult(info map[string]interface{}) {
	level, exists := info["level"]
	if !exists {
		fmt.Println("session: onResult: no 'level' in info object")
		return
	}
	code, exists := info["code"]
	if !exists {
		fmt.Println("session: onResult: no 'code' in info object")
		return
	}

	if level == "error" {
		fmt.Printf("session: onResult error: %+v\n", info)
		session.active = false
		return
	}
	if level == "warning" {
		fmt.Printf("session: onResult warning: %+v\n", info)
	}

	switch code {
	case "NetConnection.Connect.Success":
		session.messageManager.requestCreateStream(2)
	}
}

func (session *Session) onStreamBegin() {
	session.messageManager.requestPlay(session.streamKey)
}

func (session *Session) onStatus(info map[string]interface{}) {
	level, exists := info["level"]
	if !exists {
		fmt.Println("session: onStatus: no 'level' in info object")
		return
	}
	code, exists := info["code"]
	if !exists {
		fmt.Println("session: onStatus: no 'code' in info object")
		return
	}
	if level == "error" {
		fmt.Printf("session: onStatus error: %s, %+v\n", code, info)
		session.active = false
		return
	}

	if level == "warning" {
		fmt.Printf("session: onStatus warning: %s, %+v\n", code, info)
	}

	switch code {
	case "NetStream.Play.Start":
		fmt.Println("received NetStream.Play.Start")
		// TODO: set up transcoders
	default:
		fmt.Println("session: onStatus: received unknown code:", code)
	}
}

func (session *Session) GetID() string {
	return session.sessionID
}

func (session *Session) onWindowAckSizeReached(sequenceNumber uint32) {
}

func (session *Session) onConnect(csID uint32, transactionID float64, data map[string]interface{}) {
	fmt.Println("onConnect called")
	session.storeMetadata(data)
	// If the app name to connect is App (whatever the user specifies in the config, ie. "app", "app/publish"),
	if session.app == config.App {
		// Initiate connect sequence
		// As per the specification, after the connect command, the server sends the protocol message Window Acknowledgment Size
		session.messageManager.sendWindowAckSize(config.DefaultClientWindowSize)
		// After sending the window ack size message, the server sends the set peer bandwidth message
		session.messageManager.sendSetPeerBandWidth(config.DefaultClientWindowSize, LimitDynamic)
		// Send the User Control Message to begin stream with stream ID = DefaultPublishStream (which is 0)
		// Subsequent messages sent by the client will have stream ID = DefaultPublishStream, until another sendBeginStream message is sent
		session.messageManager.sendBeginStream(config.DefaultPublishStream)
		// Send Set Chunk Size message
		session.messageManager.sendSetChunkSize(config.DefaultChunkSize)
		// Send Connect Success response
		session.messageManager.sendConnectSuccess(csID)
	} else {
		fmt.Println("session: user trying to connect to app \"" + session.app + "\", but the app doesn't exist. Closing connection.")
		session.active = false
	}
}

func (session *Session) storeMetadata(metadata map[string]interface{}) {
	fmt.Printf("storeMetadata: %+v\n", metadata)
	// Playback clients send other properties in the command object, such as what audio/video codecs the client supports
	session.app = metadata["app"].(string)
	if _, exists := metadata["flashVer"]; exists {
		session.flashVer = metadata["flashVer"].(string)
	} else if _, exists := metadata["flashver"]; exists {
		session.flashVer = metadata["flashver"].(string)
	}

	if _, exists := metadata["swfUrl"]; exists {
		session.flashVer = metadata["swfUrl"].(string)
	} else if _, exists := metadata["swfurl"]; exists {
		session.flashVer = metadata["swfurl"].(string)
	}

	if _, exists := metadata["tcUrl"]; exists {
		session.flashVer = metadata["tcUrl"].(string)
	} else if _, exists := metadata["tcurl"]; exists {
		session.flashVer = metadata["tcurl"].(string)
	}

	if _, exists := metadata["type"]; exists {
		session.flashVer = metadata["type"].(string)
	}
}

func (session *Session) onSetChunkSize(size uint32) {
	session.messageManager.SetChunkSize(size)
}

func (session *Session) onAbortMessage(chunkStreamId uint32) {
}

func (session *Session) onAck(sequenceNumber uint32) {
}

func (session *Session) onSetWindowAckSize(windowAckSize uint32) {
	session.messageManager.SetWindowAckSize(windowAckSize)
}

func (session *Session) onSetBandwidth(windowAckSize uint32, limitType uint8) {
	session.messageManager.SetBandwidth(windowAckSize, limitType)
}

func (session *Session) onMetadata(metadata map[string]interface{}) {
	// This is not for the RTMP server. RTMP servers don't have the option to specify callback. Only RTMP clients use this for now
	if session.OnMetadata != nil {
		session.OnMetadata(metadata)
		return
	}

	// If this is a client session, no further processing should be done. i.e: no need to broadcast, since we're only receiving data.
	if session.isClient {
		return
	}

	// TODO: broadcast metadata to client
	session.broadcaster.BroadcastMetadata(session.streamKey, metadata)
	//if config.Debug {
	//	fmt.Printf("clientMetadata %+v", session.clientMetadata)
	//}
}

func (session *Session) onReleaseStream(csID uint32, transactionID float64, args map[string]interface{}, streamKey string) {
}

func (session *Session) onFCPublish(csID uint32, transactionID float64, args map[string]interface{}, streamKey string) {
	session.messageManager.sendOnFCPublish(csID, transactionID, streamKey)
}

func (session *Session) onCreateStream(csID uint32, transactionID float64, data map[string]interface{}) {
	// data object could be nil
	session.messageManager.sendCreateStreamResponse(csID, transactionID, data)
	session.messageManager.sendBeginStream(uint32(config.DefaultStreamID))
}

func (session *Session) onPublish(transactionId float64, args map[string]interface{}, streamKey string, publishingType string) {
	// TODO: Handle things like look up the user's stream key, check if it's valid.
	// TODO: For example: twitch returns "Publishing live_user_<username>" in the description.
	// TODO: Handle things like recording into a file if publishingType = "record" or "append". Or always record?

	session.streamKey = streamKey
	session.publishingType = publishingType

	session.messageManager.sendStatusMessage("status", "NetStream.Publish.Start", "Publishing live_user_<x>")
	session.isPublisher = true
	session.broadcaster.RegisterPublisher(streamKey)
}

func (session *Session) onFCUnpublish(args map[string]interface{}, streamKey string) {
}

func (session *Session) onDeleteStream(args map[string]interface{}, streamID float64) {
}

func (session *Session) SendEndOfStream() {
	session.messageManager.sendStatusMessage("status", "NetStream.Play.Stop", "Stopped playing stream.")
}

func (session *Session) onCloseStream(csID uint32, transactionId float64, args map[string]interface{}) {

}

// audioData is the full payload (it has the audio headers at the beginning of the payload), for easy forwarding
// If format == audio.AAC, audioData will contain AACPacketType at index 1
func (session *Session) onAudioMessage(format audio.Format, sampleRate audio.SampleRate, sampleSize audio.SampleSize, channels audio.Channel, payload []byte, timestamp uint32) {
	// This is not for the RTMP server. RTMP servers don't have the option to specify callback. Only RTMP clients use this for now
	if session.OnAudio != nil {
		session.OnAudio(format, sampleRate, sampleSize, channels, payload, timestamp)
		return
	}

	// If this is a client session, no further processing should be done. i.e: no need to broadcast, since we're only receiving data.
	// TODO: maybe caching the AAC/ACV sequence headers will be necessary
	if session.isClient {
		return
	}

	// Cache aac sequence header to send to play back clients when they connect
	if format == audio.AAC && audio.AACPacketType(payload[1]) == audio.AACSequenceHeader {
		session.broadcaster.SetAacSequenceHeaderForPublisher(session.streamKey, payload)
	}
	session.broadcaster.BroadcastAudio(session.streamKey, payload, timestamp)
}

// videoData is the full payload (it has the video headers at the beginning of the payload), for easy forwarding
func (session *Session) onVideoMessage(frameType video.FrameType, codec video.Codec, payload []byte, timestamp uint32) {
	// This is not for the RTMP server. RTMP servers don't have the option to specify callback. Only RTMP clients use this for now
	if session.OnVideo != nil {
		session.OnVideo(frameType, codec, payload, timestamp)
		return
	}

	// If this is a client session, no further processing should be done. i.e: no need to broadcast, since we're only receiving data.
	// TODO: maybe caching the AAC/ACV sequence headers will be necessary
	if session.isClient {
		return
	}

	// cache avc sequence header to send to playback clients when they connect
	if codec == video.H264 && video.AVCPacketType(payload[1]) == video.AVCSequenceHeader {
		session.broadcaster.SetAvcSequenceHeaderForPublisher(session.streamKey, payload)
	}
	session.broadcaster.BroadcastVideo(session.streamKey, payload, timestamp)
}

func (session *Session) onPlay(streamKey string, startTime float64) {
	session.streamKey = streamKey

	if !session.broadcaster.StreamExists(streamKey) {
		session.messageManager.sendStatusMessage("error", "NetStream.Play.StreamNotFound", "not_found", streamKey)
		return
	}
	session.messageManager.sendStatusMessage("status", "NetStream.Play.Start", "Playing stream for live_user_<x>")
	avcSeqHeader := session.broadcaster.GetAvcSequenceHeaderForPublisher(streamKey)
	if avcSeqHeader != nil {
		if config.Debug {
			fmt.Printf("sending video onPlay, sequence header with timestamp: 0, body size: %d\n", len(avcSeqHeader))
		}
		session.messageManager.sendVideo(avcSeqHeader, 0)
	}

	aacSeqHeader := session.broadcaster.GetAacSequenceHeaderForPublisher(streamKey)
	if aacSeqHeader != nil {
		if config.Debug {
			fmt.Printf("sending audio onPlay, sequence header with timestamp: 0, body size: %d\n", len(aacSeqHeader))
		}
		session.messageManager.sendAudio(aacSeqHeader, 0)
	}

	session.isPlayer = true
	err := session.broadcaster.RegisterSubscriber(streamKey, session)
	if err != nil {
		// TODO: send failure response to client
	}
}

func (session *Session) SendAudio(audio []byte, timestamp uint32) {
	session.messageManager.sendAudio(audio, timestamp)
}

func (session *Session) SendVideo(video []byte, timestamp uint32) {
	session.messageManager.sendVideo(video, timestamp)
}

func (session *Session) SendMetadata(metadata map[string]interface{}) {
	session.messageManager.sendMetadata(metadata)
}
