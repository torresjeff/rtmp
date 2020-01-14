package rtmp

type Context struct {
	publishers       map[uint32]*Session
	subscribers map[uint32][]*Session
	numberOfSessions uint32
}

func NewContext() *Context {
	return &Context{
		publishers: make(map[uint32]*Session),
	}
}

// Registers the session in the context to keep a reference to all open publishers
func (c *Context) RegisterSession(id uint32, session *Session) {
	c.publishers[id] = session
	c.numberOfSessions++
}

func (c *Context) DestroySession(id uint32) {
	if _, exists := c.publishers[id]; exists {
		delete(c.publishers, id)
		c.numberOfSessions--
	}
}