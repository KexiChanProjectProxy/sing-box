package cloudflared

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/gorilla/websocket"
)

const (
	defaultCloudflaredVersion = "2026.2.0"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.CloudflaredOutboundOptions](registry, C.TypeCloudflared, NewOutbound)
}

type Outbound struct {
	outbound.Adapter
	ctx     context.Context
	logger  log.ContextLogger
	dialer  N.Dialer
	version string
	hostname string
	wsURL    string
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.CloudflaredOutboundOptions) (adapter.Outbound, error) {
	if options.Hostname == "" {
		return nil, E.New("missing required field: hostname")
	}

	version := options.CloudflaredVersion
	if version == "" {
		version = defaultCloudflaredVersion
	}

	outboundDialer, err := dialer.New(ctx, options.DialerOptions, true)
	if err != nil {
		return nil, err
	}

	return &Outbound{
		Adapter:  outbound.NewAdapterWithDialerOptions(C.TypeCloudflared, tag, []string{N.NetworkTCP}, options.DialerOptions),
		ctx:      ctx,
		logger:   logger,
		dialer:   outboundDialer,
		version:  version,
		hostname: options.Hostname,
		wsURL:    "wss://" + options.Hostname,
	}, nil
}

func (o *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = o.Tag()
	metadata.Destination = destination

	o.logger.InfoContext(ctx, "outbound connection to ", destination, " via cloudflared tunnel (", o.hostname, ")")

	switch N.NetworkName(network) {
	case N.NetworkTCP:
		o.logger.DebugContext(ctx, "dialing cloudflared websocket to ", o.hostname)

		gorillaDialer := &websocket.Dialer{
			NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return o.dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
			HandshakeTimeout: 45 * time.Second,
		}

		headers := http.Header{}
		headers.Set("User-Agent", "cloudflared/"+o.version)

		wsConn, _, err := gorillaDialer.DialContext(ctx, o.wsURL, headers)
		if err != nil {
			o.logger.ErrorContext(ctx, "failed to dial cloudflared websocket to ", o.hostname, ": ", err)
			return nil, E.Cause(err, "dial cloudflared websocket to ", o.hostname)
		}

		o.logger.InfoContext(ctx, "cloudflared websocket connected to ", o.hostname)

		return newCloudflaredConn(wsConn, o.logger), nil

	default:
		return nil, E.New("cloudflared only supports TCP")
	}
}

func (o *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

// cloudflaredConn wraps a gorilla/websocket connection to implement net.Conn
// with binary message framing, matching the official cloudflared client's behavior.
type cloudflaredConn struct {
	conn   *websocket.Conn
	reader io.Reader
	logger log.ContextLogger
}

// newCloudflaredConn creates a new cloudflaredConn from a gorilla websocket connection.
func newCloudflaredConn(ws *websocket.Conn, logger log.ContextLogger) *cloudflaredConn {
	return &cloudflaredConn{conn: ws, logger: logger}
}

// Read reads from the WebSocket connection using binary message framing.
// It uses NextReader to get the message reader, exactly like the official client.
func (c *cloudflaredConn) Read(p []byte) (n int, err error) {
	for {
		if c.reader == nil {
			c.logger.Trace("waiting for next websocket message")
			var messageType int
			messageType, c.reader, err = c.conn.NextReader()
			if err != nil {
				c.logger.Debug("websocket read error: ", err)
				return 0, err
			}
			c.logger.Trace("received websocket message, type=", messageType)
			if messageType != websocket.BinaryMessage {
				return 0, net.ErrClosed
			}
		}
		n, err = c.reader.Read(p)
		if err == io.EOF {
			c.logger.Trace("websocket message fully consumed, waiting for next")
			c.reader = nil
			if n > 0 {
				return n, nil // return data, swallow EOF
			}
			continue // no data, get next message
		}
		if n > 0 {
			c.logger.Trace("read ", n, " bytes from websocket")
		}
		if err != nil {
			c.logger.Debug("websocket read error: ", err)
		}
		return n, err
	}
}

// Write writes to the WebSocket connection using binary message framing.
// It uses WriteMessage with BinaryMessage type, exactly like the official client.
func (c *cloudflaredConn) Write(p []byte) (n int, err error) {
	c.logger.Trace("writing ", len(p), " bytes to websocket")
	err = c.conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		c.logger.Debug("websocket write error: ", err)
		return 0, err
	}
	return len(p), nil
}

// Close closes the WebSocket connection.
func (c *cloudflaredConn) Close() error {
	c.logger.Debug("closing cloudflared websocket connection")
	return c.conn.Close()
}

// SetDeadline sets both read and write deadlines.
func (c *cloudflaredConn) SetDeadline(t time.Time) error {
	if err := c.conn.SetReadDeadline(t); err != nil {
		return err
	}
	return c.conn.SetWriteDeadline(t)
}

// SetReadDeadline sets the read deadline.
func (c *cloudflaredConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline.
func (c *cloudflaredConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

// LocalAddr returns the local network address.
func (c *cloudflaredConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (c *cloudflaredConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}
