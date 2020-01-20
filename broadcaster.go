package rtmp

import "fmt"

// A subscriber gets sent audio, video and data messages that flow in a particular stream (identified with streamKey)
type Subscriber interface {
	sendAudio(audio []byte, timestamp uint32)
	sendVideo(video []byte, timestamp uint32)
	// TODO: data messages as well?
	GetID() uint32
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
	// TODO: send stop message to all subscribers
	return b.context.DestroyPublisher(streamKey)
}

func (b* Broadcaster) RegisterSubscriber(streamKey string, subscriber Subscriber) error {
	return b.context.RegisterSubscriber(streamKey, subscriber)
}

func (b *Broadcaster) StreamExists(streamKey string) bool {
	return b.context.StreamExists(streamKey)
}


func (b *Broadcaster) broadcastAudio(streamKey string, audio []byte, timestamp uint32) error {
	// TODO: should this use goroutines? Problem with using goroutines: if the publisher's session is destroyed, the context will delete the session ID,
	// TODO: trying to get the publisher's sessionID from the context will result in a panic (because we're trying to access a key that doesn't exist).
	// TODO: This could lead to some playback clients receiving the last audio/video message sent by the publisher, and other clients won't get it,
	// TODO: because deleting the sessionID could happen in the middle of broadcasting.

	subscribers, err := b.context.GetSubscribersForStream(streamKey)
	if err != nil {
		return err
	}
	for _, sub := range subscribers {
		sub.sendAudio(audio, timestamp)
	}
	return nil
}

func (b *Broadcaster) broadcastVideo(streamKey string, video []byte, timestamp uint32) error {
	// TODO: should this use goroutines? Problem with using goroutines: if the publisher's session is destroyed, the context will delete the session ID,
	// TODO: trying to get the publisher's sessionID from the context will result in a panic (because we're trying to access a key that doesn't exist).
	// TODO: This could lead to some playback clients receiving the last audio/video message sent by the publisher, and other clients won't get it,
	// TODO: because deleting the sessionID could happen in the middle of broadcasting.

	subscribers, err := b.context.GetSubscribersForStream(streamKey)
	if err != nil {
		fmt.Println("broadcaster: error getting subscribers for stream, " + err.Error())
		return err
	}

	for _, sub := range subscribers {
		sub.sendVideo(video, timestamp)
	}
	return nil
}

func (b *Broadcaster) DestroySubscriber(streamKey string, sessionID uint32) error {
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