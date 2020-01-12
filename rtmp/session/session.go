package session

import (
	"bufio"
	"fmt"
	"github.com/torresjeff/rtmp-server/config"
	"github.com/torresjeff/rtmp-server/utils"
	"io"
	"net"
)

const RtmpVersion3 = 0x03

// Represents a connection made with the RTMP server where messages are exchanged between client/server.
type Session struct {
	id     uint32
	conn   net.Conn
	socket *bufio.ReadWriter

	//handshakeState HandshakeState
	c1             []byte
	s1             []byte

	parser *ChunkHandler
}

func NewSession(conn *net.Conn) *Session {
	session := &Session{
		id:             GenerateSessionId(),
		conn:           *conn,
		socket:         bufio.NewReadWriter(bufio.NewReaderSize(*conn, config.BuffioSize), bufio.NewWriterSize(*conn, config.BuffioSize)),
		//handshakeState: RtmpHandshakeUninit,
	}
	session.parser = NewChunkHandler(session.socket)
	RegisterSession(session.id, session)
	return session
}

// Run performs the initial handshake and starts receiving streams of data.
func (session *Session) Run() error {
	// Perform handshake
	err := session.Handshake()
	if err != nil {
		session.conn.Close()
		return err
	}

	if config.Debug {
		fmt.Println("Handshake completed successfully")
	}

	// After handshake, start reading chunks
	for {
		// This would be readMessage instead, when implementing a client.
		// readMessage would assemble a message from multiple chunks.
		if err := session.readChunk(); err == io.EOF {
			session.conn.Close()
			return nil
		} else if err != nil {
			return err
		}
	}
}

func (session *Session) Handshake() error {
	var err error
	if err = session.readC0C1(); err != nil {
		return err
	}
	if err = session.sendS0S1S2(); err != nil {
		return err
	}
	if err = session.readC2(); err != nil {
		return err
	}

	return nil
}

func (session *Session) GetID() uint32 {
	return session.id
}

func (session *Session) send(bytes []byte) error {
	if _, err := session.socket.Write(bytes); err != nil {
		return err
	}
	if err := session.socket.Flush(); err != nil {
		return err
	}
	return nil
}

func (session *Session) write(bytes []byte) error {
	if _, err := session.socket.Write(bytes); err != nil {
		return err
	}
	return nil
}

func (session *Session) flush() error {
	if err := session.socket.Flush(); err != nil {
		return err
	}
	return nil
}

func (session *Session) read(bytes []byte) error {
	if _, err := session.socket.Read(bytes); err != nil {
		return err
	}
	return nil
}

func (session *Session) readC0C1() error {
	var c0c1 [1537]byte
	err := session.read(c0c1[:])
	if err != nil {
		return err
	}
	// Extract c1 message (which starts at byte 1) and store it for future use (sending it in S2)
	session.c1 = c0c1[1:]
	return nil
}

func (session *Session) readC2() error {
	var c2 [1536]byte
	if err := session.read(c2[:]); err != nil {
		return err
	}
	return nil
}

// Sends the s0, s1, and s2 sequence
func (session *Session) sendS0S1S2() error {
	var s0s1s2 [1 + 2*1536]byte
	var err error
	// s0 message is stored in byte 0
	s0s1s2[0] = RtmpVersion3
	// s1 message is stored in bytes 1-1536
	if err = session.generateS1(s0s1s2[1:1537]); err != nil {
		return err
	}
	// s2 message is stored in bytes 1537-3073
	if err = session.generateS2(s0s1s2[1537:]); err != nil {
		return err
	}

	return session.send(s0s1s2[:])
}

// Generates an S1 message and stores it in s1
func (session *Session) generateS1(s1 []byte) error {
	// the s1 byte array is zero-initialized, since we didn't modify it, we're sending our time as 0
	err := utils.GenerateRandomDataFromBuffer(s1[8:])
	if err != nil {
		return err
	}
	session.s1 = s1[:]
	return nil
}


// Generates an S1 message and stores it in s2. The S2 message is an echo of the C1 message
func (session *Session) generateS2(s2 []byte) error {
	copy(s2[:], session.c1)
	return nil
}

func (session *Session) readChunk() error {
	// TODO: every time a chunk is read, update the number of read bytes
	var err error
	chunkHeader, err := session.parser.ReadChunkHeader()
	if err != nil {
		return err
	}

	_, err = session.parser.ReadChunkData(chunkHeader)
	if err != nil {
		return err
	}
	//if config.Debug{
	//	fmt.Println("chunkBasicHeader", chunkHeader.BasicHeader)
	//	fmt.Println("chunkMessageHeader", chunkHeader.MessageHeader)
	//	fmt.Println("chunkData", chunkData)
	//}


	return nil
}