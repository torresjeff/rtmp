package rtmp

import (
	"errors"
	"fmt"
	"github.com/torresjeff/rtmp/config"
	"sync"
)

type ContextStore interface {
	RegisterPublisher(streamKey string) error
	DestroyPublisher(streamKey string) error
	RegisterSubscriber(streamKey string, subscriber Subscriber) error
	GetSubscribersForStream(streamKey string) ([]Subscriber, error)
	DestroySubscriber(streamKey string, sessionID string) error
	StreamExists(streamKey string) bool
	SetAvcSequenceHeaderForPublisher(streamKey string, payload []byte)
	GetAvcSequenceHeaderForPublisher(streamKey string) []byte
	SetAacSequenceHeaderForPublisher(streamKey string, payload []byte)
	GetAacSequenceHeaderForPublisher(streamKey string) []byte
}

type InMemoryContext struct {
	ContextStore
	subMutex               sync.RWMutex
	subscribers            map[string][]Subscriber
	seqMutex               sync.RWMutex
	avcSequenceHeaderCache map[string][]byte
	aacSequenceHeaderCache map[string][]byte
	numberOfSessions       uint32
}

var StreamNotFound error = errors.New("StreamNotFound")

func NewInMemoryContext() *InMemoryContext {
	return &InMemoryContext{
		subscribers:            make(map[string][]Subscriber),
		avcSequenceHeaderCache: make(map[string][]byte),
		aacSequenceHeaderCache: make(map[string][]byte),
	}
}

// Registers the session in the broadcaster to keep a reference to all open subscribers
func (c *InMemoryContext) RegisterPublisher(streamKey string) error {
	// Assume there will be a small amount of subscribers (ie. a few instances of ffmpeg that transcode our audio/video)
	c.subMutex.Lock()
	c.subscribers[streamKey] = make([]Subscriber, 0, 5)
	if config.Debug {
		fmt.Println("context: registered publisher with stream key", streamKey)
	}
	c.numberOfSessions++
	c.subMutex.Unlock()
	return nil
}

func (c *InMemoryContext) DestroyPublisher(streamKey string) error {
	c.subMutex.Lock()
	defer c.subMutex.Unlock()
	if _, exists := c.subscribers[streamKey]; exists {
		delete(c.subscribers, streamKey)
		c.numberOfSessions--
	}
	return nil
}

func (c *InMemoryContext) RegisterSubscriber(streamKey string, subscriber Subscriber) error {
	c.subMutex.Lock()
	defer c.subMutex.Unlock()
	// If a stream with the key exists, then register the subscriber
	if _, exists := c.subscribers[streamKey]; exists {
		c.subscribers[streamKey] = append(c.subscribers[streamKey], subscriber)
		return nil
	}
	// If no stream was found with that stream key, send an error
	return StreamNotFound
}

func (c *InMemoryContext) StreamExists(streamKey string) bool {
	c.subMutex.RLock()
	defer c.subMutex.RUnlock()
	_, exists := c.subscribers[streamKey]
	return exists
}

func (c *InMemoryContext) GetSubscribersForStream(streamKey string) ([]Subscriber, error) {
	// We could add a cache check if this context got the subscribers from a DB rather than from memory
	c.subMutex.RLock()
	defer c.subMutex.RUnlock()
	// If a stream with the key exists, then return its subscribers
	if subscribers, exists := c.subscribers[streamKey]; exists {
		return subscribers, nil
	}
	// If no stream was found with that stream key, send an error
	return nil, StreamNotFound
}

func (c *InMemoryContext) DestroySubscriber(streamKey string, sessionID string) error {
	c.subMutex.Lock()
	defer c.subMutex.Unlock()
	subscribers, exists := c.subscribers[streamKey]
	if !exists {
		return nil
	}
	numberOfSubs := len(subscribers)
	for i, sub := range subscribers {
		if sub.GetID() == sessionID {
			// Swap the subscriber we're deleting with the last element, to avoid having to delete (more efficient)
			subscribers[i] = subscribers[numberOfSubs-1]
			c.subscribers[streamKey] = subscribers[:numberOfSubs-1]
		}
	}
	return nil
}

func (c *InMemoryContext) SetAvcSequenceHeaderForPublisher(streamKey string, payload []byte) {
	c.seqMutex.Lock()
	c.avcSequenceHeaderCache[streamKey] = payload
	c.seqMutex.Unlock()
}

func (c *InMemoryContext) GetAvcSequenceHeaderForPublisher(streamKey string) []byte {
	c.seqMutex.RLock()
	defer c.seqMutex.RUnlock()
	return c.avcSequenceHeaderCache[streamKey]
}

func (c *InMemoryContext) SetAacSequenceHeaderForPublisher(streamKey string, payload []byte) {
	c.seqMutex.Lock()
	c.aacSequenceHeaderCache[streamKey] = payload
	c.seqMutex.Unlock()
}

func (c *InMemoryContext) GetAacSequenceHeaderForPublisher(streamKey string) []byte {
	c.seqMutex.RLock()
	defer c.seqMutex.RUnlock()
	return c.aacSequenceHeaderCache[streamKey]
}
