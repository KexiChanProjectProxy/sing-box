package firefoxvpn

import (
	"context"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/sagernet/sing/common/baderror"
)

type connectConnOptions struct {
	reader      io.ReadCloser
	writer      *io.PipeWriter
	requestBody *io.PipeReader
	cancel      contextCancelFunc
	localAddr   net.Addr
	remoteAddr  net.Addr
	release     func()
}

type contextCancelFunc = context.CancelFunc

type connectConn struct {
	reader      io.ReadCloser
	writer      *io.PipeWriter
	requestBody *io.PipeReader
	cancel      contextCancelFunc
	localAddr   net.Addr
	remoteAddr  net.Addr
	release     func()

	closeOnce sync.Once
	closeErr  error

	deadlineMu    sync.Mutex
	readDeadline  time.Time
	writeDeadline time.Time
}

func newConnectConn(options connectConnOptions) net.Conn {
	return &connectConn{
		reader:      options.reader,
		writer:      options.writer,
		requestBody: options.requestBody,
		cancel:      options.cancel,
		localAddr:   options.localAddr,
		remoteAddr:  options.remoteAddr,
		release:     options.release,
	}
}

func (c *connectConn) Read(buffer []byte) (int, error) {
	return c.runWithDeadline(c.readDeadlineValue(), func() (int, error) {
		n, err := c.reader.Read(buffer)
		return n, baderror.WrapH2(err)
	})
}

func (c *connectConn) Write(buffer []byte) (int, error) {
	return c.runWithDeadline(c.writeDeadlineValue(), func() (int, error) {
		n, err := c.writer.Write(buffer)
		return n, baderror.WrapH2(err)
	})
}

func (c *connectConn) Close() error {
	return c.closeWithError(nil)
}

func (c *connectConn) LocalAddr() net.Addr {
	if c.localAddr != nil {
		return c.localAddr
	}
	return tunnelAddr("firefox-vpn-h2")
}

func (c *connectConn) RemoteAddr() net.Addr {
	if c.remoteAddr != nil {
		return c.remoteAddr
	}
	return tunnelAddr("firefox-vpn-h2")
}

func (c *connectConn) SetDeadline(deadline time.Time) error {
	c.deadlineMu.Lock()
	defer c.deadlineMu.Unlock()
	c.readDeadline = deadline
	c.writeDeadline = deadline
	return nil
}

func (c *connectConn) SetReadDeadline(deadline time.Time) error {
	c.deadlineMu.Lock()
	defer c.deadlineMu.Unlock()
	c.readDeadline = deadline
	return nil
}

func (c *connectConn) SetWriteDeadline(deadline time.Time) error {
	c.deadlineMu.Lock()
	defer c.deadlineMu.Unlock()
	c.writeDeadline = deadline
	return nil
}

func (c *connectConn) readDeadlineValue() time.Time {
	c.deadlineMu.Lock()
	defer c.deadlineMu.Unlock()
	return c.readDeadline
}

func (c *connectConn) writeDeadlineValue() time.Time {
	c.deadlineMu.Lock()
	defer c.deadlineMu.Unlock()
	return c.writeDeadline
}

func (c *connectConn) runWithDeadline(deadline time.Time, fn func() (int, error)) (int, error) {
	if deadline.IsZero() {
		return fn()
	}
	timeout := time.Until(deadline)
	if timeout <= 0 {
		_ = c.closeWithError(os.ErrDeadlineExceeded)
		return 0, os.ErrDeadlineExceeded
	}
	type ioResult struct {
		n   int
		err error
	}
	resultCh := make(chan ioResult, 1)
	go func() {
		n, err := fn()
		resultCh <- ioResult{n: n, err: err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case result := <-resultCh:
		return result.n, result.err
	case <-timer.C:
		_ = c.closeWithError(os.ErrDeadlineExceeded)
		return 0, os.ErrDeadlineExceeded
	}
}

func (c *connectConn) closeWithError(closeReason error) error {
	c.closeOnce.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		if c.writer != nil {
			if closeReason != nil {
				_ = c.writer.CloseWithError(closeReason)
			} else {
				_ = c.writer.Close()
			}
		}
		if c.requestBody != nil {
			if closeReason != nil {
				_ = c.requestBody.CloseWithError(closeReason)
			} else {
				_ = c.requestBody.Close()
			}
		}
		if c.reader != nil {
			c.closeErr = c.reader.Close()
		}
		if c.release != nil {
			c.release()
		}
		if c.closeErr == nil && closeReason != nil {
			c.closeErr = closeReason
		}
	})
	return c.closeErr
}

type tunnelAddr string

func (a tunnelAddr) Network() string { return "tcp" }

func (a tunnelAddr) String() string { return string(a) }
