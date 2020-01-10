package session

import (
	"math/rand"
	"time"
)

var sessions map[uint32]*Session = make(map[uint32]*Session)

var numberOfSessions uint32
var source rand.Source = rand.NewSource(time.Now().UnixNano())
var generator *rand.Rand = rand.New(source)


// Generates a unique ID when a session is created to identify it.
func GenerateSessionId() uint32 {
	numberOfSessions++
	// TODO: guarantee randomness with crypto/rand?
	return generator.Uint32()
}

// Registers the session in the context to keep a reference to all open sessions
func RegisterSession(id uint32, session *Session) {
	sessions[id] = session
}