package rtmp

import "fmt"

// A subscriber gets sent audio, video and data messages that flow in a particular stream (identified with streamKey)
type Subscriber interface {
	sendAudio(audio []byte)
	sendVideo(video []byte)
	// TODO: data messages as well
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

func (b* Broadcaster) RegisterSubscriber(streamKey string, subscriber Subscriber) error {
	return b.context.RegisterSubscriber(streamKey, subscriber)
}



func (b *Broadcaster) broadcastAudio(streamKey string, audio []byte) error {
	// TODO: should this use goroutines? Problem with using goroutines: if the publisher's session is destroyed, the context will delete the session ID,
	// TODO: trying to get the publisher's sessionID from the context will result in a panic (because we're trying to access a key that doesn't exist).
	// TODO: This could lead to some playback clients receiving the last audio/video message sent by the publisher, and other clients won't get it,
	// TODO: because deleting the sessionID could happen in the middle of broadcasting.

	subscribers, err := b.context.GetSubscribersForStream(streamKey)
	if err != nil {
		return err
	}
	for _, sub := range subscribers {
		sub.sendAudio(audio)
	}
	return nil
}

func (b *Broadcaster) broadcastVideo(streamKey string, video []byte) error {
	// TODO: should this use goroutines? Problem with using goroutines: if the publisher's session is destroyed, the context will delete the session ID,
	// TODO: trying to get the publisher's sessionID from the context will result in a panic (because we're trying to access a key that doesn't exist).
	// TODO: This could lead to some playback clients receiving the last audio/video message sent by the publisher, and other clients won't get it,
	// TODO: because deleting the sessionID could happen in the middle of broadcasting.

	// We could add a cache here if the context was on a db rather than on memory, to avoid having to fetch all subscribers

	subscribers, err := b.context.GetSubscribersForStream(streamKey)
	if err != nil {
		fmt.Println("broadcaster: error getting subscribers for stream, " + err.Error())
		return err
	}

	for _, sub := range subscribers {
		sub.sendVideo(video)
	}
	return nil
}