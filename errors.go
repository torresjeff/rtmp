package rtmp

import "errors"

var ErrNilWriter = errors.New("Expected *bufio.Writer to be non-nil, but got a nil value")
var ErrNilReader = errors.New("Expected *bufio.Reader to be non-nil, but got a nil value")
