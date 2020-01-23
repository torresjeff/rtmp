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

func ClientHandshake(reader *bufio.Reader, writer *bufio.Writer) error {
	c1, err := sendC0C1(writer)
	if err != nil {
		return err
	}
	s1, s2, err := readS0S1S2(reader)
	if err != nil {
		return err
	}
	if bytes.Compare(c1, s2) != 0 {
		return ErrWrongS2Message
	}
	err = sendC2(writer, s1)
	if err != nil {
		return err
	}

	//message := generateConnectRequest(3, 1, map[string]interface{}{
	//	"app": "app",
	//	"flashVer": "LNX 9,0,124,2",
	//	"tcUrl": "rtmp://192.168.1.2/app",
	//	"fpad": false,
	//	"capabilities": 15,
	//	"audioCodecs": 4071,
	//	"videoCodecs": 252,
	//	"videoFunction": 1,
	//})
	//send(writer, message)
	return nil
}

func sendC2(writer *bufio.Writer, s1 []byte) error {
	var c2 [1536]byte
	err := generateEcho(c2[:], s1)
	if err != nil {
		return err
	}
	err = send(writer, c2[:])
	if err != nil {
		return err
	}
	return nil
}

// Returns s1 and s2
func readS0S1S2(reader *bufio.Reader) ([]byte, []byte, error) {
	var s0s1s2 [1 + 2*1536]byte

	if _, err := io.ReadFull(reader, s0s1s2[:]); err != nil {
		return nil, nil, err
	}

	if s0s1s2[0] != RtmpVersion3 {
		return nil, nil, ErrUnsupportedRTMPVersion
	}

	// Return s1, s2, and nil error
	return s0s1s2[1:1537], s0s1s2[1537:], nil
}

// Returns the C1 message that was sent
func sendC0C1(writer *bufio.Writer) ([]byte, error) {
	var c0c1 [1537]byte
	// c0
	c0c1[0] = RtmpVersion3
	// c1
	err := generateRandomData(c0c1[1:])
	if err != nil {
		return nil, err
	}

	err = send(writer, c0c1[:])
	if err != nil {
		return nil, err
	}

	return c0c1[1:], nil
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
	if err = generateRandomData(s0s1s2[1:1537]); err != nil {
		return  nil, err
	}
	// s2 message is stored in bytes 1537-3073
	if err = generateEcho(s0s1s2[1537:], c1); err != nil {
		return nil, err
	}
	err = send(writer, s0s1s2[:])
	if err != nil {
		return nil, err
	}
	return s0s1s2[1:1537], nil
}

// Generates an S1 message (random data)
func generateRandomData(s1 []byte) error {
	// the s1 byte array is zero-initialized, since we didn't modify it, we're sending our time as 0
	err := rand.GenerateRandomDataFromBuffer(s1[8:])
	if err != nil {
		return err
	}
	return nil
}


func generateEcho(target []byte, source []byte) error {
	copy(target[:], source)
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
