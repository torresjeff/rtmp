package rtmp

import (
	"bufio"
	"fmt"
	"github.com/torresjeff/rtmp/audio"
	"github.com/torresjeff/rtmp/config"
	"github.com/torresjeff/rtmp/video"
	"io"
	"net"
	"strings"
)

type surroundSound struct {
	stereoSound bool
	twoPointOneSound bool
	threePointOneSound bool
	fourPointZeroSound bool
	fourPointOneSound bool
	fivePointOneSound bool
	sevenPointOneSound bool
}

type clientMetadata struct {
	duration      float64
	fileSize      float64
	width         float64
	height        float64
	videoCodecID  string
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

// Media Server interface defines the callbacks that are called when a message is received
type MediaServer interface {
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
	onSetDataFrame(metadata map[string]interface{})
	onFCUnpublish(args map[string]interface{}, streamKey string)
	onDeleteStream(args map[string]interface{}, streamID float64)
	onCloseStream(csID uint32, transactionId float64, args map[string]interface{})
	onAudioMessage(format audio.Format, sampleRate audio.SampleRate, sampleSize audio.SampleSize, channels audio.Channel, payload []byte, timestamp uint32)
	onVideoMessage(frameType video.FrameType, codec video.Codec, payload []byte, timestamp uint32)
	onPlay(streamKey string, startTime float64)
}

// Represents a connection made with the RTMP server where messages are exchanged between client/server.
type Session struct {
	MediaServer
	sessionID      uint32
	conn           net.Conn
	socketr        *bufio.Reader
	socketw        *bufio.Writer
	clientMetadata clientMetadata
	broadcaster    *Broadcaster // broadcasts audio/video messages to playback clients subscribed to a stream
	active         bool         // true if the session is active

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
	isPublisher bool
	isPlayer bool
}

func NewSession(sessionID uint32, conn *net.Conn, b *Broadcaster) *Session {
	session := &Session{
		sessionID:         sessionID,
		conn:              *conn,
		socketr:           bufio.NewReaderSize(*conn, config.BuffioSize),
		socketw: bufio.NewWriterSize(*conn, config.BuffioSize),
		broadcaster:       b,
		active:            true,
	}
	chunkHandler := NewChunkHandler(session.socketr, session.socketw)
	session.messageManager = NewMessageManager(session, chunkHandler)
	return session
}

// Run performs the initial handshake and starts receiving streams of data.
func (session *Session) Run() error {
	// Perform handshake
	err := Handshake(session.socketr, session.socketw)
	if err == io.EOF {
		return session.conn.Close()
	} else if err != nil {
		session.conn.Close()
		fmt.Println("session: error in handshake")
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
			if err = session.messageManager.nextMessage(); err == io.EOF {
				fmt.Println("session: closing connection")
				return session.conn.Close()
			} else if err != nil {
				fmt.Println("session: closing connection")
				session.conn.Close()
				return err
			}
		} else {
			// Session is over, finish the session
			return session.conn.Close()
		}
	}
}

func (session *Session) GetID() uint32 {
	return session.sessionID
}

func (session *Session) onWindowAckSizeReached(sequenceNumber uint32) {
}

func (session *Session) onConnect(csID uint32, transactionID float64, data map[string]interface{}) {
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
		session.conn.Close()
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

func (session *Session) onSetDataFrame(metadata map[string]interface{}) {
	// Put all keys in lowercase to handle different formats for each client uniformly
	for key, val := range metadata {
		metadata[strings.ToLower(key)] = val
	}

	if val, exists := metadata["duration"]; exists {
		session.clientMetadata.duration = val.(float64)
	}
	if val, exists := metadata["filesize"]; exists {
		session.clientMetadata.fileSize = val.(float64)
	}
	if val, exists := metadata["width"]; exists {
		session.clientMetadata.width = val.(float64)
	}
	if val, exists := metadata["height"]; exists {
		session.clientMetadata.height = val.(float64)
	}
	if val, exists := metadata["videocodecid"]; exists {
		switch val.(type) {
		case float64:
			session.clientMetadata.nVideoCodecID = val.(float64)
		case string:
			session.clientMetadata.videoCodecID = val.(string)
		}
	}
	if val, exists := metadata["videodatarate"]; exists {
		session.clientMetadata.videoDataRate = val.(float64)
	}
	if val, exists := metadata["framerate"]; exists {
		session.clientMetadata.frameRate = val.(float64)
	}
	if val, exists := metadata["audiocodecid"]; exists {
		switch val.(type) {
		case float64:
			session.clientMetadata.nAudioCodecID = val.(float64)
		case string:
			session.clientMetadata.audioCodecID = val.(string)
		}
	}
	if val, exists := metadata["audiodatarate"]; exists {
		session.clientMetadata.audioDataRate = val.(float64)
	}
	if val, exists := metadata["audiosamplerate"]; exists {
		session.clientMetadata.audioSampleRate = val.(float64)
	}
	if val, exists := metadata["audiosamplesize"]; exists {
		session.clientMetadata.audioSampleSize = val.(float64)
	}
	if val, exists := metadata["audiochannels"]; exists {
		session.clientMetadata.audioChannels = val.(float64)
	}
	if val, exists := metadata["stereo"]; exists {
		session.clientMetadata.sound.stereoSound = val.(bool)
	}
	if val, exists := metadata["2.1"]; exists {
		session.clientMetadata.sound.twoPointOneSound = val.(bool)
	}
	if val, exists := metadata["3.1"]; exists {
		session.clientMetadata.sound.threePointOneSound = val.(bool)
	}
	if val, exists := metadata["4.0"]; exists {
		session.clientMetadata.sound.fourPointZeroSound = val.(bool)
	}
	if val, exists := metadata["4.1"]; exists {
		session.clientMetadata.sound.fourPointOneSound = val.(bool)
	}
	if val, exists := metadata["5.1"]; exists {
		session.clientMetadata.sound.fivePointOneSound = val.(bool)
	}
	if val, exists := metadata["7.1"]; exists {
		session.clientMetadata.sound.sevenPointOneSound = val.(bool)
	}
	if val, exists := metadata["encoder"]; exists {
		session.clientMetadata.encoder = val.(string)
	}

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
	// Cache aac sequence header to send to play back clients when they connect
	if format == audio.AAC && audio.AACPacketType(payload[1]) == audio.AACSequenceHeader {
		session.broadcaster.SetAacSequenceHeaderForPublisher(session.streamKey, payload)
	}
	session.broadcaster.BroadcastAudio(session.streamKey, payload, timestamp)
}

// videoData is the full payload (it has the video headers at the beginning of the payload), for easy forwarding
func (session *Session) onVideoMessage(frameType video.FrameType, codec video.Codec, payload []byte, timestamp uint32) {
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