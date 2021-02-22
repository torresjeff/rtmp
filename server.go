package rtmp

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/torresjeff/rtmp/config"
	"go.uber.org/zap"
	"net"
)

// Server represents the RTMP server, where a client/app can stream media to. The server listens for incoming connections.
type Server struct {
	Addr string
	// TODO: create Logger interface to not depend on zap directly
	Logger *zap.Logger
	// TODO: should probably add something like maxConns
}

// Listen starts the server and listens for any incoming connections. If no Addr (host:port) has been assigned to the server, ":1935" is used.
func (s *Server) Listen() error {
	if s.Addr == "" {
		s.Addr = ":" + config.DefaultPort
	}

	tcpAddress, err := net.ResolveTCPAddr("tcp", s.Addr)
	if err != nil {
		err = errors.Errorf("[server] error resolving tcp address: %s", err)
		return err
	}

	// Start listening on the specified address
	listener, err := net.ListenTCP("tcp", tcpAddress)
	if err != nil {
		return err
	}

	s.Logger.Info(fmt.Sprint("[server] Listening on ", s.Addr))

	for {
		conn, err := listener.Accept()
		if err != nil {
			s.Logger.Error(fmt.Sprint("[server] Error accepting incoming connection ", err))
			continue
		}

		s.Logger.Info(fmt.Sprint("[server] Accepted incoming connection from ", conn.RemoteAddr().String()))

		go func(conn net.Conn) {
			defer conn.Close()

			// TODO: Instantiate all necessary stuff: chunk stream, message stream, readers, writers, etc.
		}(conn)

	}

}
