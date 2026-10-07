// Package cloudflarewarp implements the client side of Cloudflare WARP's
// MASQUE tunnel: an HTTP/3 extended CONNECT with the "cf-connect-ip" protocol
// that carries raw IP packets as HTTP datagrams.
package cloudflarewarp

import (
	"context"
	"errors"
	"strconv"

	"github.com/sagernet/sing/common/buf"
)

// PacketHeadroom is the space a caller must reserve in front of packets passed
// to Session.WritePacket for the datagram context ID.
const PacketHeadroom = 1

// Transport opens tunnel sessions. HTTP/3 is the only implementation today;
// the interface leaves room for Cloudflare's HTTP/2 fallback.
type Transport interface {
	Connect(ctx context.Context) (Session, error)
}

type Session interface {
	// WritePacket sends one IP packet. The buffer must have PacketHeadroom
	// bytes of headroom; the caller keeps ownership.
	WritePacket(buffer *buf.Buffer) error
	// ReadPacket returns the next IP packet received from the tunnel.
	ReadPacket() ([]byte, error)
	// Done is closed when the session ends; Err then reports why.
	Done() <-chan struct{}
	Err() error
	Close() error
}

// ErrEnrollmentRejected reports that Cloudflare refused the device key.
var ErrEnrollmentRejected = errors.New("device credentials rejected by Cloudflare")

// ErrSessionClosed is reported when the session was closed locally.
var ErrSessionClosed = errors.New("tunnel session closed")

type PacketTooLargeError struct {
	MaxPacketSize int
}

func (e *PacketTooLargeError) Error() string {
	return "IP packet exceeds the tunnel datagram limit of " + strconv.Itoa(e.MaxPacketSize) + " bytes"
}
