package rtmp

import (
	"errors"
	"fmt"
)

type ContextStore interface {
	RegisterPublisher(streamKey string) error
	DestroyPublisher(streamKey string) error
	RegisterSubscriber(streamKey string, subscriber Subscriber) error
	GetSubscribersForStream(streamKey string) ([]Subscriber, error)
	SetAvcSequenceHeaderForPublisher(streamKey string, payload []byte)
	GetAvcSequenceHeaderForPublisher(streamKey string) []byte
}

type InMemoryContext struct {
	ContextStore
	subscribers      map[string][]Subscriber
	numberOfSessions uint32
	avcSequenceHeaderCache map[string][]byte
}

var StreamNotFound error = errors.New("StreamNotFound")

func NewInMemoryContext() *InMemoryContext {
	return &InMemoryContext{
		subscribers: make(map[string][]Subscriber),
		avcSequenceHeaderCache: make(map[string][]byte),
	}
}

// Registers the session in the broadcaster to keep a reference to all open subscribers
func (c *InMemoryContext) RegisterPublisher(streamKey string) error {
	// Assume there will be a small amount of subscribers (ie. a few instances of ffmpeg that transcode our audio/video)
	c.subscribers[streamKey] = make([]Subscriber, 0, 5)
	fmt.Println("registered publisher with stream key", streamKey)
	c.numberOfSessions++
	return nil
}

func (c *InMemoryContext) DestroyPublisher(streamKey string) error {
	if _, exists := c.subscribers[streamKey]; exists {
		delete(c.subscribers, streamKey)
		c.numberOfSessions--
	}
	return nil
}

func (c *InMemoryContext) RegisterSubscriber(streamKey string, subscriber Subscriber) error {
	// If a stream with the key exists, then register the subscriber
	if _, exists := c.subscribers[streamKey]; exists {
		c.subscribers[streamKey] = append(c.subscribers[streamKey], subscriber)
		return nil
	}
	// If no stream was found with that stream key, send an error
	return StreamNotFound
}

func (c *InMemoryContext) GetSubscribersForStream(streamKey string) ([]Subscriber, error) {
	// We could add a cache check if this context got the subscribers from a DB rather than from memory

	// If a stream with the key exists, then return its subscribers
	if subscribers, exists := c.subscribers[streamKey]; exists {
		return subscribers, nil
	}
	// If no stream was found with that stream key, send an error
	return nil, StreamNotFound
}

func (c *InMemoryContext) SetAvcSequenceHeaderForPublisher(streamKey string, payload []byte) {
	c.avcSequenceHeaderCache[streamKey] = payload
}

func (c *InMemoryContext) GetAvcSequenceHeaderForPublisher(streamKey string) []byte {
	// TODO: handle cases where cache doesn't exist
	return c.avcSequenceHeaderCache[streamKey]
}