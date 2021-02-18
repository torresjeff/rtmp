package rtmp

import "fmt"

// A subscriber gets sent audio, video and data messages that flow in a particular stream (identified with streamKey)
type Subscriber interface {
	SendAudio(audio []byte, timestamp uint32)
	SendVideo(video []byte, timestamp uint32)
	SendMetadata(metadata map[string]interface{})
	GetID() string
	SendEndOfStream()
}

type Broadcaster struct {
	context ContextStore
}

func NewBroadcaster(context ContextStore) *Broadcaster {
	return &Broadcaster{
		context: context,
	}
}

func (b *Broadcaster) RegisterPublisher(streamKey string) error {
	return b.context.RegisterPublisher(streamKey)
}

func (b *Broadcaster) DestroyPublisher(streamKey string) error {
	return b.context.DestroyPublisher(streamKey)
}

func (b *Broadcaster) RegisterSubscriber(streamKey string, subscriber Subscriber) error {
	return b.context.RegisterSubscriber(streamKey, subscriber)
}

func (b *Broadcaster) StreamExists(streamKey string) bool {
	return b.context.StreamExists(streamKey)
}

func (b *Broadcaster) BroadcastAudio(streamKey string, audio []byte, timestamp uint32) error {
	subscribers, err := b.context.GetSubscribersForStream(streamKey)
	if err != nil {
		fmt.Println("broadcaster: BroadcastAudio: error getting subscribers for stream, " + err.Error())
		return err
	}
	for _, sub := range subscribers {
		sub.SendAudio(audio, timestamp)
	}
	return nil
}

func (b *Broadcaster) BroadcastVideo(streamKey string, video []byte, timestamp uint32) error {
	subscribers, err := b.context.GetSubscribersForStream(streamKey)
	if err != nil {
		fmt.Println("broadcaster: BroadcastVideo: error getting subscribers for stream, " + err.Error())
		return err
	}

	for _, sub := range subscribers {
		sub.SendVideo(video, timestamp)
	}
	return nil
}

func (b *Broadcaster) DestroySubscriber(streamKey string, sessionID string) error {
	return b.context.DestroySubscriber(streamKey, sessionID)
}

func (b *Broadcaster) SetAvcSequenceHeaderForPublisher(streamKey string, payload []byte) {
	b.context.SetAvcSequenceHeaderForPublisher(streamKey, payload)
}

func (b *Broadcaster) GetAvcSequenceHeaderForPublisher(streamKey string) []byte {
	return b.context.GetAvcSequenceHeaderForPublisher(streamKey)
}

func (b *Broadcaster) SetAacSequenceHeaderForPublisher(streamKey string, payload []byte) {
	b.context.SetAacSequenceHeaderForPublisher(streamKey, payload)
}

func (b *Broadcaster) GetAacSequenceHeaderForPublisher(streamKey string) []byte {
	return b.context.GetAacSequenceHeaderForPublisher(streamKey)
}

func (b *Broadcaster) BroadcastEndOfStream(streamKey string) {
	subscribers, err := b.context.GetSubscribersForStream(streamKey)
	if err != nil {
		fmt.Println("broadcaster: broadcast end of stream: error getting subscribers for stream, " + err.Error())
		return
	}

	for _, sub := range subscribers {
		sub.SendEndOfStream()
	}
}

func (b *Broadcaster) BroadcastMetadata(streamKey string, metadata map[string]interface{}) error {
	subscribers, err := b.context.GetSubscribersForStream(streamKey)
	if err != nil {
		fmt.Println("broadcaster: BroadcastVideo: error getting subscribers for stream, " + err.Error())
		return err
	}

	for _, sub := range subscribers {
		sub.SendMetadata(metadata)
	}
	return nil
}
