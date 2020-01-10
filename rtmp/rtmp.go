package rtmp

import (
	"fmt"
	"github.com/torresjeff/rtmp-server/config"
	"github.com/torresjeff/rtmp-server/rtmp/session"
	"net"
)

type Server struct {
	Addr string
}

// Run starts the server and listens for any incoming connections. If no Addr (host:port) has been assigned to the server, ":1935" is used.
func (server *Server) Run() error {
	if server.Addr == "" {
		server.Addr = ":1935"
	}

	tcpAddress, err := net.ResolveTCPAddr("tcp", server.Addr);
	if err != nil {
		err = fmt.Errorf("rtmp: ListenAndServer: %s", err)
		return err
	}

	// Start listening on the specified address
	listener, err := net.ListenTCP("tcp", tcpAddress)
	if err != nil {
		return err
	}

	if config.Debug {
		fmt.Println("rtmp: server: listening on", server.Addr)
	}

	// Loop infinitely, accepting any incoming connection
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		if config.Debug {
			fmt.Println("accepted incoming connection from", conn.RemoteAddr().String())
		}

		// Create a new session from the new connection (basically a wrapper of the connection + other data)
		sess := session.NewSession(&conn)
		// TODO: fill in any other data for the session that is needed

		go func () {
			err := sess.Run()
			if config.Debug {
				fmt.Println("rtmp: server: session closed err:", err)
			}
		}()

	}

}