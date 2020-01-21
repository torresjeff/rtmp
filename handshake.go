package rtmp

import (
	"bufio"
	"bytes"
	"errors"
	"github.com/torresjeff/rtmp/rand"
	"io"
)

var ErrUnsupportedRTMPVersion error = errors.New("The version of RTMP is not supported")
var ErrWrongC2Message error = errors.New("s1 and c2 handshake messages do not match")
const RtmpVersion3 = 3

func Handshake(reader *bufio.Reader, writer *bufio.Writer) error {
	c1, err := readC0C1(reader)
	if err != nil {
		return err
	}
	s1, err := sendS0S1S2(writer, c1)
	if err != nil {
		return err
	}
	c2, err := readC2(reader)
	if err != nil {
		return err
	}

	if bytes.Compare(s1, c2) != 0 {
		return ErrWrongC2Message
	}

	return nil
}

// If successful returns the C1 handshake data (random data sent by the client), it does not return c0 + c1.
func readC0C1(reader *bufio.Reader) ([]byte, error) {
	var c0c1 [1537]byte

	if _, err := io.ReadFull(reader, c0c1[:]); err != nil {
		return nil, err
	}

	if c0c1[0] != RtmpVersion3 {
		return nil, ErrUnsupportedRTMPVersion
	}

	// Returns c1 message
	return c0c1[1:], nil
}

// Returns the C2 message
func readC2(reader *bufio.Reader) ([]byte, error) {
	var c2 [1536]byte
	if _, err := io.ReadFull(reader, c2[:]); err != nil {
		return nil, err
	}
	return c2[:], nil
}

// Sends the s0, s1, and s2 sequence and returns the s1 message that was generated
func sendS0S1S2(writer *bufio.Writer, c1 []byte) ([]byte, error) {
	var s0s1s2 [1 + 2*1536]byte
	var err error
	// s0 message is stored in byte 0
	s0s1s2[0] = RtmpVersion3
	// s1 message is stored in bytes 1-1536
	if err = generateS1(s0s1s2[1:1537]); err != nil {
		return  nil, err
	}
	// s2 message is stored in bytes 1537-3073
	if err = generateS2(s0s1s2[1537:], c1); err != nil {
		return nil, err
	}
	err = send(writer, s0s1s2[:])
	if err != nil {
		return nil, err
	}
	return s0s1s2[1:1537], nil
}

// Generates an S1 message (random data)
func generateS1(s1 []byte) error {
	// the s1 byte array is zero-initialized, since we didn't modify it, we're sending our time as 0
	err := rand.GenerateRandomDataFromBuffer(s1[8:])
	if err != nil {
		return err
	}
	return nil
}


func generateS2(s2 []byte, c1 []byte) error {
	copy(s2[:], c1)
	return nil
}

func send(writer *bufio.Writer, bytes []byte) error {
	if _, err := writer.Write(bytes); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	return nil
}
