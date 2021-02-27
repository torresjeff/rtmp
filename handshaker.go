package rtmp

import (
	"bytes"
	"errors"
	"io"
)

const Version3 = 3

var ErrUnsupportedVersion = errors.New("unsupported rtmp version")
var ErrS2DoesNotMatchC1 = errors.New("s2 message sent by server does not match c1 message sent by client")
var ErrC2DoesNotMatchS1 = errors.New("c2 message sent by client does not match s1 message sent by server")

type Handshaker interface {
	Handshake(reader io.Reader, writer WriteFlusher) error
}

type ClientHandshaker struct {
	Handshaker
	// fills the byte slice with random data, it should always return the length of the slice of bytes and a nil error
	randomNumberGenerator func([]byte) (int, error)
}

func NewClientHandshaker(randomNumberGenerator func([]byte) (int, error)) *ClientHandshaker {
	return &ClientHandshaker{
		randomNumberGenerator: randomNumberGenerator,
	}
}

func (c *ClientHandshaker) Handshake(reader io.Reader, writer WriteFlusher) error {
	c1, err := c.sendC0C1(writer)
	if err != nil {
		return err
	}

	s1, s2, err := c.readS0S1S2(reader)
	if err != nil {
		return err
	}

	if bytes.Compare(c1, s2) != 0 {
		return ErrS2DoesNotMatchC1
	}

	err = c.sendC2(writer, s1)

	return err
}

// sendC0C1 writes C0 + C1 messages to writer and returns the C1 message that was written.
func (c *ClientHandshaker) sendC0C1(writer WriteFlusher) ([]byte, error) {
	var c0c1 [1537]byte
	// c0
	c0c1[0] = Version3
	// c1
	_, err := c.randomNumberGenerator(c0c1[1:])
	if err != nil {
		return nil, err
	}

	_, err = writer.Write(c0c1[:])
	if err != nil {
		return nil, err
	}

	err = writer.Flush()
	if err != nil {
		return nil, err
	}

	return c0c1[1:], nil
}

// readS0S1S2 reads the S0 + S1 + S2 messages from the reader and returns the S1 and S2 messages.
func (c *ClientHandshaker) readS0S1S2(reader io.Reader) ([]byte, []byte, error) {
	var s0s1s2 [1 + 2*1536]byte

	if _, err := io.ReadFull(reader, s0s1s2[:]); err != nil {
		return nil, nil, err
	}

	if s0s1s2[0] != Version3 {
		return nil, nil, ErrUnsupportedVersion
	}

	// Return s1, s2, and nil error
	return s0s1s2[1:1537], s0s1s2[1537:], nil
}

func (c *ClientHandshaker) sendC2(writer WriteFlusher, s1 []byte) error {
	var c2 [1536]byte

	copy(c2[:], s1)
	_, err := writer.Write(c2[:])
	if err != nil {
		return err
	}

	err = writer.Flush()
	return err
}

type ServerHandshaker struct {
	Handshaker
	randomNumberGenerator func([]byte) (int, error)
}

func NewServerHandshaker(randomNumberGenerator func([]byte) (int, error)) *ServerHandshaker {
	return &ServerHandshaker{
		randomNumberGenerator: randomNumberGenerator,
	}
}

func (s *ServerHandshaker) Handshake(reader io.Reader, writer WriteFlusher) error {
	c1, err := s.readC0C1(reader)
	if err != nil {
		return err
	}

	s1, err := s.sendS0S1S2(writer, c1)
	if err != nil {
		return err
	}

	c2, err := s.readC2(reader)

	if bytes.Compare(s1, c2) != 0 {
		return ErrC2DoesNotMatchS1
	}

	return nil
}

// readC0C1 reads the C0 + C1 messages from the reader and returns the C1 message that was read.
func (s *ServerHandshaker) readC0C1(reader io.Reader) ([]byte, error) {
	var c0c1 [1537]byte

	if _, err := io.ReadFull(reader, c0c1[:]); err != nil {
		return nil, err
	}

	if c0c1[0] != Version3 {
		return nil, ErrUnsupportedVersion
	}

	// Returns c1 message
	return c0c1[1:], nil
}

// sendS0S1S2 writes the S0 + S1 + S2 messages to the writer and returns the S1 messages.
func (s *ServerHandshaker) sendS0S1S2(writer WriteFlusher, c1 []byte) ([]byte, error) {
	var s0s1s2 [1 + 2*1536]byte
	// s0 message is stored in byte 0
	s0s1s2[0] = Version3
	// s1 message is stored in bytes 1-1536
	_, err := s.randomNumberGenerator(s0s1s2[1:1537])
	if err != nil {
		return nil, err
	}
	// s2 message is stored in bytes 1537-3073
	copy(s0s1s2[1537:], c1)
	_, err = writer.Write(s0s1s2[:])
	if err != nil {
		return nil, err
	}

	err = writer.Flush()
	if err != nil {
		return nil, err
	}

	return s0s1s2[1:1537], nil
}

func (s *ServerHandshaker) readC2(reader io.Reader) ([]byte, error) {
	var c2 [1536]byte
	if _, err := io.ReadFull(reader, c2[:]); err != nil {
		return nil, err
	}
	return c2[:], nil
}
