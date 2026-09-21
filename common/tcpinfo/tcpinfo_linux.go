package tcpinfo

import (
	"net"
	"syscall"
	"time"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/control"

	"golang.org/x/sys/unix"
)

// Supported reports whether Read can return data on this platform.
const Supported = true

// Read returns TCP_INFO for the TCP socket under conn, unwrapping TLS and other
// wrappers that expose their underlying connection. It returns false when conn
// does not lead to a TCP socket.
func Read(conn net.Conn) (Info, bool) {
	syscallConn, isSyscallConn := common.Cast[syscall.Conn](conn)
	if !isSyscallConn {
		return Info{}, false
	}
	if _, isTCP := common.Cast[*net.TCPConn](conn); !isTCP {
		return Info{}, false
	}
	info, err := control.Conn0(syscallConn, func(fd uintptr) (*unix.TCPInfo, error) {
		return unix.GetsockoptTCPInfo(int(fd), unix.IPPROTO_TCP, unix.TCP_INFO)
	})
	if err != nil || info == nil {
		return Info{}, false
	}
	return Info{
		RTT:             time.Duration(info.Rtt) * time.Microsecond,
		RTTVar:          time.Duration(info.Rttvar) * time.Microsecond,
		DataSegmentsOut: uint64(info.Data_segs_out),
		Retransmits:     uint64(info.Total_retrans),
		DeliveryRate:    info.Delivery_rate,
		BytesSent:       info.Bytes_sent,
	}, true
}
