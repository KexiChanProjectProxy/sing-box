package v2rayxhttp

import (
	"io"
	"net"
	"time"

	"github.com/sagernet/sing/common"
)

// splitConn implements net.Conn with separate reader and writer streams
type splitConn struct {
	writer     io.WriteCloser
	reader     io.ReadCloser
	remoteAddr net.Addr
	localAddr  net.Addr
	onClose    func()
}

func (c *splitConn) Read(b []byte) (n int, err error) {
	return c.reader.Read(b)
}

func (c *splitConn) Write(b []byte) (n int, err error) {
	return c.writer.Write(b)
}

func (c *splitConn) Close() error {
	if c.onClose != nil {
		c.onClose()
	}
	return common.Close(c.writer, c.reader)
}

func (c *splitConn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *splitConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *splitConn) SetDeadline(t time.Time) error {
	// Not supported for split connections
	return nil
}

func (c *splitConn) SetReadDeadline(t time.Time) error {
	// Not supported for split connections
	return nil
}

func (c *splitConn) SetWriteDeadline(t time.Time) error {
	// Not supported for split connections
	return nil
}

// Ensure splitConn implements net.Conn
var _ net.Conn = (*splitConn)(nil)
