// Package tcpinfo reads the kernel's view of a TCP connection.
package tcpinfo

import "time"

// Info is the part of TCP_INFO that describes path quality.
//
// Counters are cumulative over the connection's life. A field the running
// kernel does not report is zero; the Has fields say which groups are real.
type Info struct {
	RTT    time.Duration
	RTTVar time.Duration
	// MinRTT is the lowest RTT seen, the path without queueing (Linux 4.6).
	HasMinRTT bool
	MinRTT    time.Duration
	// RcvRTT is the receiver's own RTT estimate.
	RcvRTT time.Duration
	// DataSegmentsOut counts data segments sent, retransmissions included.
	DataSegmentsOut uint64
	DataSegmentsIn  uint64
	// Retransmits counts retransmitted segments over the connection's life.
	Retransmits uint64
	// DeliveryRate is the kernel's most recent delivery rate estimate, in
	// bytes per second. It is application limited most of the time, which
	// AppLimited says of the most recent sample (Linux 4.9).
	HasDeliveryRate bool
	DeliveryRate    uint64
	AppLimited      bool
	// PacingRate is the current pacing rate in bytes per second.
	PacingRate uint64
	SndCwnd    uint32
	SndMSS     uint32
	// Unacked counts segments sent and not yet acknowledged, and
	// NotSentBytes the bytes written by the application that TCP has not
	// sent yet; both are zero once everything written is delivered.
	Unacked       uint32
	NotSentBytes  uint32
	BytesAcked    uint64
	BytesReceived uint64

	// HasChrono is set when the kernel reports how long the connection had
	// data to send (Linux 4.10). BusyTime is that time; RwndLimited and
	// SndbufLimited are the parts of it spent waiting on the peer's receive
	// window and on the local send buffer. The rest was limited by the
	// congestion window or pacing, which is to say by the network.
	// The time is counted in jiffies, so it is only as fine as the kernel's
	// tick, which is 1 to 10 milliseconds.
	HasChrono     bool
	BusyTime      time.Duration
	RwndLimited   time.Duration
	SndbufLimited time.Duration
	// Delivered counts segments delivered (Linux 4.18).
	Delivered uint64

	// HasRetransBytes is set when the kernel reports bytes sent and
	// retransmitted (Linux 4.19).
	HasRetransBytes bool
	BytesSent       uint64
	BytesRetrans    uint64
	// DSACKDups counts retransmissions the receiver already had, which were
	// not losses (Linux 5.0).
	HasDSACK  bool
	DSACKDups uint64

	// HasRTOStats is set when the kernel reports retransmission timeouts
	// (Linux 6.7).
	HasRTOStats bool
	TotalRTO    uint64

	// CongestionControl names the algorithm, and BBR holds its estimates
	// when the algorithm is BBR.
	CongestionControl string
	BBR               *BBRInfo
}

// NetworkLimited is the time the connection had data to send and was held
// back by neither the peer nor the local buffer.
func (i Info) NetworkLimited() time.Duration {
	limited := i.BusyTime - i.RwndLimited - i.SndbufLimited
	if limited < 0 {
		return 0
	}
	return limited
}

// BBRInfo is BBR's model of the path.
type BBRInfo struct {
	// Bandwidth is the estimated bottleneck bandwidth in bytes per second, a
	// windowed maximum of delivery rates.
	Bandwidth uint64
	MinRTT    time.Duration
	// PacingGain and CwndGain are the current gains, 1 being unity.
	PacingGain float64
	CwndGain   float64
}
