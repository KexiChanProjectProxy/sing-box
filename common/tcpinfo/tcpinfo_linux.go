package tcpinfo

import (
	"encoding/binary"
	"net"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/control"

	"golang.org/x/sys/unix"
)

// Supported reports whether Read can return data on this platform.
const Supported = true

// tcpInfoSize is the size of the struct this package knows; an older kernel
// returns a prefix of it.
const tcpInfoSize = uint32(unsafe.Sizeof(unix.TCPInfo{}))

// Where each group of fields ends; a kernel that returns less does not
// report the group.
var (
	minRTTEnd       = uint32(unsafe.Offsetof(unix.TCPInfo{}.Min_rtt) + 4)
	deliveryRateEnd = uint32(unsafe.Offsetof(unix.TCPInfo{}.Delivery_rate) + 8)
	chronoEnd       = uint32(unsafe.Offsetof(unix.TCPInfo{}.Sndbuf_limited) + 8)
	deliveredEnd    = uint32(unsafe.Offsetof(unix.TCPInfo{}.Delivered) + 4)
	retransBytesEnd = uint32(unsafe.Offsetof(unix.TCPInfo{}.Bytes_retrans) + 8)
	dsackEnd        = uint32(unsafe.Offsetof(unix.TCPInfo{}.Dsack_dups) + 4)
	rtoStatsEnd     = uint32(unsafe.Offsetof(unix.TCPInfo{}.Total_rto) + 2)
)

// appLimitedBit is tcpi_delivery_rate_app_limited, the first bitfield of its
// byte, which compilers place at the low bit on little endian targets and at
// the high bit on big endian ones.
var appLimitedBit = func() byte {
	if binary.NativeEndian.Uint16([]byte{1, 0}) == 1 {
		return 0x01
	}
	return 0x80
}()

// bbrUnit is BBR's fixed point unity for gains.
const bbrUnit = 256

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
	info, err := control.Conn0(syscallConn, func(fd uintptr) (Info, error) {
		return readFD(int(fd))
	})
	if err != nil {
		return Info{}, false
	}
	return info, true
}

func readFD(fd int) (Info, error) {
	var raw unix.TCPInfo
	size := tcpInfoSize
	if err := getsockopt(fd, unix.TCP_INFO, unsafe.Pointer(&raw), &size); err == unix.ENOSYS {
		// 32-bit x86 kernels before Linux 4.3 only take socket calls through
		// socketcall, which the x/sys wrapper uses; it does not return the
		// length, so only the fields every such kernel has are kept.
		legacy, legacyErr := unix.GetsockoptTCPInfo(fd, unix.IPPROTO_TCP, unix.TCP_INFO)
		if legacyErr != nil {
			return Info{}, legacyErr
		}
		raw, size = *legacy, minRTTEnd-1
	} else if err != nil {
		return Info{}, err
	}
	// The application limited flag is a bitfield in the byte after the
	// window scales, which unix.TCPInfo leaves as padding.
	flags := (*[8]byte)(unsafe.Pointer(&raw))[7]
	info := Info{
		RTT:             time.Duration(raw.Rtt) * time.Microsecond,
		RTTVar:          time.Duration(raw.Rttvar) * time.Microsecond,
		RcvRTT:          time.Duration(raw.Rcv_rtt) * time.Microsecond,
		DataSegmentsOut: uint64(raw.Data_segs_out),
		DataSegmentsIn:  uint64(raw.Data_segs_in),
		Retransmits:     uint64(raw.Total_retrans),
		PacingRate:      raw.Pacing_rate,
		SndCwnd:         raw.Snd_cwnd,
		SndMSS:          raw.Snd_mss,
		Unacked:         raw.Unacked,
		NotSentBytes:    raw.Notsent_bytes,
		BytesAcked:      raw.Bytes_acked,
		BytesReceived:   raw.Bytes_received,
	}
	if size >= minRTTEnd {
		info.HasMinRTT = true
		info.MinRTT = time.Duration(raw.Min_rtt) * time.Microsecond
	}
	if size >= deliveryRateEnd {
		info.HasDeliveryRate = true
		info.DeliveryRate = raw.Delivery_rate
		info.AppLimited = flags&appLimitedBit != 0
	}
	if size >= chronoEnd {
		info.HasChrono = true
		info.BusyTime = time.Duration(raw.Busy_time) * time.Microsecond
		info.RwndLimited = time.Duration(raw.Rwnd_limited) * time.Microsecond
		info.SndbufLimited = time.Duration(raw.Sndbuf_limited) * time.Microsecond
	}
	if size >= deliveredEnd {
		info.Delivered = uint64(raw.Delivered)
	}
	if size >= retransBytesEnd {
		info.HasRetransBytes = true
		info.BytesSent = raw.Bytes_sent
		info.BytesRetrans = raw.Bytes_retrans
	}
	if size >= dsackEnd {
		info.HasDSACK = true
		info.DSACKDups = uint64(raw.Dsack_dups)
	}
	if size >= rtoStatsEnd {
		info.HasRTOStats = true
		info.TotalRTO = uint64(raw.Total_rto)
	}
	if name, err := unix.GetsockoptString(fd, unix.IPPROTO_TCP, unix.TCP_CONGESTION); err == nil {
		info.CongestionControl = name
		if strings.HasPrefix(name, "bbr") {
			info.BBR = readBBR(fd)
		}
	}
	return info, nil
}

func readBBR(fd int) *BBRInfo {
	var raw unix.TCPBBRInfo
	size := uint32(unsafe.Sizeof(raw))
	if err := getsockopt(fd, unix.TCP_CC_INFO, unsafe.Pointer(&raw), &size); err != nil || size < uint32(unsafe.Sizeof(raw)) {
		return nil
	}
	return &BBRInfo{
		Bandwidth:  uint64(raw.Bw_hi)<<32 | uint64(raw.Bw_lo),
		MinRTT:     time.Duration(raw.Min_rtt) * time.Microsecond,
		PacingGain: float64(raw.Pacing_gain) / bbrUnit,
		CwndGain:   float64(raw.Cwnd_gain) / bbrUnit,
	}
}

// getsockopt reads an IPPROTO_TCP option into value, setting size to what the
// kernel wrote.
func getsockopt(fd int, option int, value unsafe.Pointer, size *uint32) error {
	_, _, errno := unix.Syscall6(unix.SYS_GETSOCKOPT, uintptr(fd), uintptr(unix.IPPROTO_TCP), uintptr(option), uintptr(value), uintptr(unsafe.Pointer(size)), 0)
	if errno != 0 {
		return errno
	}
	return nil
}
