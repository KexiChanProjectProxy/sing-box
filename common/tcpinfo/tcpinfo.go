// Package tcpinfo reads the kernel's view of a TCP connection.
package tcpinfo

import "time"

// Info is the part of TCP_INFO that describes path quality.
type Info struct {
	RTT    time.Duration
	RTTVar time.Duration
	// DataSegmentsOut counts data segments sent, retransmissions included.
	DataSegmentsOut uint64
	// Retransmits counts retransmitted segments over the connection's life.
	Retransmits uint64
	// DeliveryRate is the kernel's most recent delivery rate estimate, in
	// bytes per second. It is application limited most of the time.
	DeliveryRate uint64
	BytesSent    uint64
}
