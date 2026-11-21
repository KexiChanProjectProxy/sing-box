//go:build linux

package peek

import (
	"io"
	"net"
	"syscall"
	"time"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/unix"
)

func clientHello(conn net.Conn, timeout time.Duration) (net.Conn, bool, error) {
	// Only the standard library's socket types are peeked in place: a
	// wrapper that implements syscall.Conn may hold bytes it has already
	// read from the socket, which MSG_PEEK would not see.
	var syscallConn syscall.Conn
	switch c := conn.(type) {
	case *net.TCPConn:
		syscallConn = c
	case *net.UnixConn:
		syscallConn = c
	default:
		return readClientHello(conn, timeout)
	}
	rawConn, err := syscallConn.SyscallConn()
	if err != nil {
		return conn, false, err
	}
	if timeout > 0 {
		err = conn.SetReadDeadline(time.Now().Add(timeout))
		if err != nil {
			return conn, false, err
		}
	}
	var (
		prefix  [ClientHelloPrefixLength]byte
		result  Result
		peekErr error
	)
	err = rawConn.Read(func(fd uintptr) bool {
		var n int
		for {
			n, _, peekErr = unix.Recvfrom(int(fd), prefix[:], unix.MSG_PEEK)
			if peekErr != unix.EINTR {
				break
			}
		}
		switch {
		case peekErr == unix.EAGAIN:
			// Nothing yet: wait for the netpoller.
			peekErr = nil
			return false
		case peekErr != nil:
			return true
		case n == 0:
			peekErr = io.EOF
			return true
		}
		result = ClassifyClientHello(prefix[:n])
		if result != Undecided {
			return true
		}
		// MSG_PEEK cannot see a close while bytes are queued, and the
		// close may have arrived with them, in the event Read already
		// consumed. The edge-triggered netpoller would not wake us again,
		// so check for it before waiting. Only an incomplete prefix that
		// still looks like a ClientHello gets here.
		if peerClosed(fd) {
			peekErr = io.ErrUnexpectedEOF
			return true
		}
		return false
	})
	if timeout > 0 {
		deadlineErr := conn.SetReadDeadline(time.Time{})
		if err == nil && peekErr == nil && deadlineErr != nil {
			return conn, false, deadlineErr
		}
	}
	if peekErr != nil {
		return conn, false, peekErr
	}
	if err != nil {
		if E.IsTimeout(err) {
			// Too slow to tell: not TLS.
			return conn, false, nil
		}
		return conn, false, err
	}
	return conn, result == TLS, nil
}

// peerClosed reports whether the peer has shut down its side of the
// connection, or the socket has failed.
func peerClosed(fd uintptr) bool {
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLRDHUP}}
	n, err := unix.Poll(fds, 0)
	return err == nil && n > 0 && fds[0].Revents&(unix.POLLRDHUP|unix.POLLHUP|unix.POLLERR) != 0
}
