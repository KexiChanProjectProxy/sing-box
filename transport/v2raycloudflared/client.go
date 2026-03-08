package v2raycloudflared

import (
	"context"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/gorilla/websocket"
)

const defaultCloudflaredVersion = "2026.2.0"

var _ adapter.V2RayClientTransport = (*Client)(nil)

type Client struct {
	dialer  N.Dialer
	wsURL   string
	version string
}

func NewClient(_ context.Context, dialer N.Dialer, _ M.Socksaddr, options option.V2RayCloudflaredOptions, _ tls.Config) (adapter.V2RayClientTransport, error) {
	if options.Hostname == "" {
		return nil, E.New("missing required field: hostname")
	}
	version := options.CloudflaredVersion
	if version == "" {
		version = defaultCloudflaredVersion
	}
	return &Client{
		dialer:  dialer,
		wsURL:   "wss://" + options.Hostname,
		version: version,
	}, nil
}

func (c *Client) DialContext(ctx context.Context) (net.Conn, error) {
	gorillaDialer := &websocket.Dialer{
		NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return c.dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
		},
		HandshakeTimeout: 45 * time.Second,
	}
	headers := http.Header{}
	headers.Set("User-Agent", "cloudflared/"+c.version)
	wsConn, _, err := gorillaDialer.DialContext(ctx, c.wsURL, headers)
	if err != nil {
		return nil, E.Cause(err, "dial cloudflared websocket")
	}
	return newCloudflaredConn(wsConn), nil
}

func (c *Client) Close() error {
	return nil
}

// cloudflaredConn wraps a gorilla/websocket connection to implement net.Conn
// with binary message framing, matching the official cloudflared client's behavior.
type cloudflaredConn struct {
	conn   *websocket.Conn
	reader io.Reader
}

func newCloudflaredConn(ws *websocket.Conn) *cloudflaredConn {
	return &cloudflaredConn{conn: ws}
}

func (c *cloudflaredConn) Read(p []byte) (n int, err error) {
	for {
		if c.reader == nil {
			var messageType int
			messageType, c.reader, err = c.conn.NextReader()
			if err != nil {
				return 0, err
			}
			if messageType != websocket.BinaryMessage {
				return 0, net.ErrClosed
			}
		}
		n, err = c.reader.Read(p)
		if err == io.EOF {
			c.reader = nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

func (c *cloudflaredConn) Write(p []byte) (n int, err error) {
	err = c.conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *cloudflaredConn) Close() error {
	return c.conn.Close()
}

func (c *cloudflaredConn) SetDeadline(t time.Time) error {
	if err := c.conn.SetReadDeadline(t); err != nil {
		return err
	}
	return c.conn.SetWriteDeadline(t)
}

func (c *cloudflaredConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *cloudflaredConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

func (c *cloudflaredConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *cloudflaredConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}
