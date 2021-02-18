package rtmp

import (
	"bufio"
	"bytes"
	"errors"
	"github.com/torresjeff/rtmp/rand"
	"io"
)

var ErrUnsupportedRTMPVersion error = errors.New("The version of RTMP is not supported")
var ErrWrongC2Message error = errors.New("server handshake: s1 and c2 handshake messages do not match")
var ErrWrongS2Message error = errors.New("client handshake: c1 and s2 handshake messages do not match")
var ErrHandshakeAlreadyCompleted error = errors.New("invalid call to perform handshake, attempted to perform " +
	"handshake more than once")

const RtmpVersion3 = 3

type Handshaker struct {
	reader             *bufio.Reader
	writer             *bufio.Writer
	handshakeCompleted bool
}

func NewHandshaker(reader *bufio.Reader, writer *bufio.Writer) *Handshaker {
	return &Handshaker{
		reader,
		writer,
		false,
	}
}

func (h *Handshaker) Handshake() error {
	if h.handshakeCompleted {
		return ErrHandshakeAlreadyCompleted
	}
	c1, err := h.readC0C1()
	if err != nil {
		return err
	}
	s1, err := h.sendS0S1S2(c1)
	if err != nil {
		return err
	}
	c2, err := h.readC2()
	if err != nil {
		return err
	}

	if bytes.Compare(s1, c2) != 0 {
		return ErrWrongC2Message
	}

	h.handshakeCompleted = true
	return nil
}

func (h *Handshaker) ClientHandshake() error {
	if h.handshakeCompleted {
		return ErrHandshakeAlreadyCompleted
	}
	c1, err := h.sendC0C1()
	if err != nil {
		return err
	}
	s1, s2, err := h.readS0S1S2()
	if err != nil {
		return err
	}
	if bytes.Compare(c1, s2) != 0 {
		return ErrWrongS2Message
	}
	err = h.sendC2(s1)
	if err != nil {
		return err
	}

	h.handshakeCompleted = true
	return nil
}

func (h *Handshaker) sendC2(s1 []byte) error {
	var c2 [1536]byte
	err := h.generateEcho(c2[:], s1)
	if err != nil {
		return err
	}
	err = h.send(c2[:])
	if err != nil {
		return err
	}
	return nil
}

// Returns s1 and s2
func (h *Handshaker) readS0S1S2() ([]byte, []byte, error) {
	var s0s1s2 [1 + 2*1536]byte

	if _, err := io.ReadFull(h.reader, s0s1s2[:]); err != nil {
		return nil, nil, err
	}

	if s0s1s2[0] != RtmpVersion3 {
		return nil, nil, ErrUnsupportedRTMPVersion
	}

	// Return s1, s2, and nil error
	return s0s1s2[1:1537], s0s1s2[1537:], nil
}

// Returns the C1 message that was sent
func (h *Handshaker) sendC0C1() ([]byte, error) {
	var c0c1 [1537]byte
	// c0
	c0c1[0] = RtmpVersion3
	// c1
	err := h.generateRandomData(c0c1[1:])
	if err != nil {
		return nil, err
	}

	err = h.send(c0c1[:])
	if err != nil {
		return nil, err
	}

	return c0c1[1:], nil
}

// If successful returns the C1 handshake data (random data sent by the client), it does not return c0 + c1.
func (h *Handshaker) readC0C1() ([]byte, error) {
	var c0c1 [1537]byte

	if _, err := io.ReadFull(h.reader, c0c1[:]); err != nil {
		return nil, err
	}

	if c0c1[0] != RtmpVersion3 {
		return nil, ErrUnsupportedRTMPVersion
	}

	// Returns c1 message
	return c0c1[1:], nil
}

// Returns the C2 message
func (h *Handshaker) readC2() ([]byte, error) {
	var c2 [1536]byte
	if _, err := io.ReadFull(h.reader, c2[:]); err != nil {
		return nil, err
	}
	return c2[:], nil
}

// Sends the s0, s1, and s2 sequence and returns the s1 message that was generated
func (h *Handshaker) sendS0S1S2(c1 []byte) ([]byte, error) {
	var s0s1s2 [1 + 2*1536]byte
	var err error
	// s0 message is stored in byte 0
	s0s1s2[0] = RtmpVersion3
	// s1 message is stored in bytes 1-1536
	if err = h.generateRandomData(s0s1s2[1:1537]); err != nil {
		return nil, err
	}
	// s2 message is stored in bytes 1537-3073
	if err = h.generateEcho(s0s1s2[1537:], c1); err != nil {
		return nil, err
	}
	err = h.send(s0s1s2[:])
	if err != nil {
		return nil, err
	}
	return s0s1s2[1:1537], nil
}

// Generates an S1 message (random data)
func (h *Handshaker) generateRandomData(s1 []byte) error {
	// the s1 byte array is zero-initialized, since we didn't modify it, we're sending our time as 0
	err := rand.GenerateCryptoSafeRandomData(s1[8:])
	if err != nil {
		return err
	}
	return nil
}

func (h *Handshaker) generateEcho(target []byte, source []byte) error {
	copy(target[:], source)
	return nil
}

func (h *Handshaker) send(bytes []byte) error {
	if _, err := h.writer.Write(bytes); err != nil {
		return err
	}
	if err := h.writer.Flush(); err != nil {
		return err
	}
	return nil
}
