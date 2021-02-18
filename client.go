package rtmp

import (
	"errors"
	"fmt"
	"github.com/torresjeff/rtmp/config"
	"net"
	"net/url"
	"strings"
)

var ErrInvalidScheme error = errors.New("invalid scheme in URL")

type Client struct {
	// Address of the RTMP server this client is connected to
	raddr      string
	app        string
	streamKey  string
	url        *url.URL
	OnAudio    AudioCallback
	OnVideo    VideoCallback
	OnMetadata MetadataCallback
}

func (c *Client) Connect(addr string) error {
	u, err := url.Parse(addr)
	if err != nil {
		return err
	}
	// Always assign rtmp as the scheme
	u.Scheme = "rtmp"
	if u.Port() == "" {
		u.Host += ":" + config.DefaultPort
	}
	c.url = u
	c.raddr = u.Host

	urlPath := u.Path
	path := strings.Split(urlPath, "/")
	// At the very least we need something in the path
	if len(path) == 0 || (len(path) == 1 && path[0] == "") {
		panic("invalid URL path")
	}
	if path[0] == "" {
		path = path[1:]
	}

	elements := len(path)
	// Treat the first part of the path as the app name
	c.app = strings.Join(path[:elements-1], "/")
	// The stream key is the last element of the path
	c.streamKey = path[elements-1]

	if config.Debug {
		fmt.Printf("app: \"%s\", streamKey: \"%s\"\n", c.app, c.streamKey)
	}
	conn, err := net.Dial("tcp", c.raddr)
	if err != nil {
		return err
	}

	if config.Debug {
		fmt.Println("client: connected to", conn.RemoteAddr().String())
	}

	client := NewClientSession(&conn, c.app, c.streamKey, c.OnAudio, c.OnVideo, c.OnMetadata)
	err = client.StartPlayback()
	if err != nil {
		return err
	}

	return nil
}
