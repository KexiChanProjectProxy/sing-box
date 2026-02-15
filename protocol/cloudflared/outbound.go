package cloudflared

import (
	"context"
	"net"
	"net/http"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/coder/websocket"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.CloudflaredOutboundOptions](registry, C.TypeCloudflared, NewOutbound)
}

type Outbound struct {
	outbound.Adapter
	ctx      context.Context
	logger   log.ContextLogger
	dialer   N.Dialer // the underlying dialer (detour-aware if detour is configured)
	hostname string    // Cloudflare Access hostname
	wsURL    string    // "wss://<hostname>" — WebSocket URL to connect to
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.CloudflaredOutboundOptions) (adapter.Outbound, error) {
	if options.Hostname == "" {
		return nil, E.New("missing required field: hostname")
	}

	outboundDialer, err := dialer.New(ctx, options.DialerOptions, true) // true = hostname is domain
	if err != nil {
		return nil, err
	}

	return &Outbound{
		Adapter:  outbound.NewAdapterWithDialerOptions(C.TypeCloudflared, tag, []string{N.NetworkTCP}, options.DialerOptions),
		ctx:      ctx,
		logger:   logger,
		dialer:   outboundDialer,
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
		// Create a custom HTTP client that uses our dialer (which may be a detour dialer)
		httpTransport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				// Parse the addr to M.Socksaddr and route through our dialer
				return o.dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
			ForceAttemptHTTP2: false,
		}
		httpClient := &http.Client{Transport: httpTransport}

		// Dial WebSocket to the Cloudflare edge
		wsConn, _, err := websocket.Dial(ctx, o.wsURL, &websocket.DialOptions{
			HTTPClient: httpClient,
		})
		if err != nil {
			return nil, E.Cause(err, "dial cloudflared websocket to ", o.hostname)
		}

		// Convert WebSocket to net.Conn using binary message framing
		// Use a background context so the connection lives beyond this function
		netConn := websocket.NetConn(context.Background(), wsConn, websocket.MessageBinary)
		return netConn, nil

	default:
		return nil, E.New("cloudflared only supports TCP")
	}
}

func (o *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}
