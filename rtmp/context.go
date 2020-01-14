package rtmp

type Context struct {
	sessions map[uint32]*Session
	numberOfSessions uint32
}

func NewContext() *Context {
	return &Context{
		sessions: make(map[uint32]*Session),
	}
}

// Registers the session in the context to keep a reference to all open sessions
func (c *Context) RegisterSession(id uint32, session *Session) {
	c.sessions[id] = session
	c.numberOfSessions++
}

func (c *Context) DestroySession(id uint32) {
	if _, exists := c.sessions[id]; exists {
		delete(c.sessions, id)
		c.numberOfSessions--
	}
}