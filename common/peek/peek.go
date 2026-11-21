// Package peek tells whether a connection starts with a TLS ClientHello
// before anything reads from it, so that one listener can hand TLS and
// non-TLS connections to different handlers.
package peek

import (
	"io"
	"net"
	"time"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
)

// Result is what the first bytes of a connection say about it.
type Result uint8

const (
	// Undecided means more bytes are needed.
	Undecided Result = iota
	// TLS means the bytes start a TLS ClientHello.
	TLS
	// NonTLS means the bytes cannot start a TLS ClientHello.
	NonTLS
)

// ClientHelloPrefixLength is the number of bytes ClassifyClientHello needs
// at most: the 5-byte record header and the handshake type.
const ClientHelloPrefixLength = 6

// maxRecordLength is the longest record crypto/tls accepts before it knows
// the version (maxCiphertext: 2^14 plus 2048 bytes of expansion). A record
// header announcing more is refused by the TLS server anyway, so such a
// connection is never a working ClientHello.
const maxRecordLength = 16384 + 2048

// ClassifyClientHello looks at the first bytes of a connection, of which b
// holds the ones read so far, and decides as soon as one of them rules out a
// ClientHello:
//
//	b[0]      == 0x16           handshake record
//	b[1]      == 0x03           record version 3.x
//	b[2]      <= 0x04           3.0 to 3.4
//	b[3:5]    <= 0x4800         record length crypto/tls accepts
//	b[5]      == 0x01           handshake type ClientHello
//
// Random bytes, such as a Shadowsocks salt, match all of them with a
// probability of about 3.3e-10.
func ClassifyClientHello(b []byte) Result {
	n := len(b)
	if n > 0 && b[0] != 0x16 {
		return NonTLS
	}
	if n > 1 && b[1] != 0x03 {
		return NonTLS
	}
	if n > 2 && b[2] > 0x04 {
		return NonTLS
	}
	if n > 3 && int(b[3]) > maxRecordLength>>8 {
		return NonTLS
	}
	if n > 4 && int(b[3])<<8|int(b[4]) > maxRecordLength {
		return NonTLS
	}
	if n > 5 && b[5] != 0x01 {
		return NonTLS
	}
	if n >= ClientHelloPrefixLength {
		return TLS
	}
	return Undecided
}

// ClientHello waits until the first bytes of conn tell whether it starts with
// a TLS ClientHello, and returns the connection to read from afterwards.
//
// On Linux, a *net.TCPConn or *net.UnixConn is inspected with MSG_PEEK and
// returned as it is, with nothing read from it. Other connections, and every
// connection on other platforms, are read from, and the bytes read are put
// back in front of the returned connection with a bufio.CachedConn.
//
// If timeout passes first, the connection is reported as not TLS. The read
// deadline of conn is cleared on return, so conn must not have one set.
//
// If the peer closes the connection or reading fails before the bytes
// decide, the error is returned with conn, and the caller should close it.
func ClientHello(conn net.Conn, timeout time.Duration) (net.Conn, bool, error) {
	return clientHello(conn, timeout)
}

// readClientHello is the portable ClientHello: it reads the bytes it needs
// and puts them back with a CachedConn.
func readClientHello(conn net.Conn, timeout time.Duration) (net.Conn, bool, error) {
	if timeout > 0 {
		err := conn.SetReadDeadline(time.Now().Add(timeout))
		if err != nil {
			return conn, false, err
		}
	}
	var (
		prefix [ClientHelloPrefixLength]byte
		n      int
		result Result
		err    error
	)
	for result == Undecided {
		var m int
		m, err = conn.Read(prefix[n:])
		n += m
		if m > 0 {
			result = ClassifyClientHello(prefix[:n])
		}
		if result != Undecided || err != nil {
			break
		}
	}
	if timeout > 0 {
		deadlineErr := conn.SetReadDeadline(time.Time{})
		if err == nil && deadlineErr != nil {
			err = deadlineErr
			result = Undecided
		}
	}
	if result == Undecided {
		switch {
		case err == nil:
		case E.IsTimeout(err):
			// Too slow to tell: not TLS.
			result = NonTLS
			err = nil
		case err == io.EOF && n > 0:
			err = io.ErrUnexpectedEOF
		}
		if err != nil {
			return conn, false, err
		}
	}
	if n > 0 {
		buffer := buf.NewSize(n)
		copy(buffer.Extend(n), prefix[:n])
		conn = bufio.NewCachedConn(conn, buffer)
	}
	return conn, result == TLS, nil
}
