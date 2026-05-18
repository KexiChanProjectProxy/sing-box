package wireguard

import (
	"context"
	"github.com/sagernet/sing-box/log"
	"net/netip"
	"time"

	"github.com/sagernet/sing-tun"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type EndpointOptions struct {
	Context      context.Context
	Logger       log.StructuredLogger
	System       bool
	Handler      tun.Handler
	UDPTimeout   time.Duration
	Dialer       N.Dialer
	CreateDialer func(interfaceName string) N.Dialer
	Name         string
	MTU          uint32
	Address      []netip.Prefix
	PrivateKey   string
	ListenPort   uint16
	ResolvePeer  func(domain string) (netip.Addr, error)
	Peers        []PeerOptions
	Workers      int
}

type PeerOptions struct {
	Endpoint                    M.Socksaddr
	PublicKey                   string
	PreSharedKey                string
	AllowedIPs                  []netip.Prefix
	PersistentKeepaliveInterval uint16
	Reserved                    []uint8
}
