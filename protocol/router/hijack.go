package router

import (
	"bufio"
	"io"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	E "github.com/sagernet/sing/common/exceptions"
)

// hijackConnection hijacks the HTTP connection from gin
// Returns a wrapped connection that preserves buffered data
func (r *Inbound) hijackConnection(c *gin.Context) (net.Conn, error) {
	// Get the hijacker interface
	hijacker, ok := c.Writer.(http.Hijacker)
	if !ok {
		return nil, E.New("response writer doesn't support hijacking")
	}

	// Hijack the connection
	conn, brw, err := hijacker.Hijack()
	if err != nil {
		return nil, E.Cause(err, "hijack connection")
	}

	// Wrap connection to preserve buffered data
	wrappedConn := &hijackedConn{
		Conn:   conn,
		reader: brw.Reader,
		writer: brw.Writer,
	}

	return wrappedConn, nil
}

// hijackedConn wraps a hijacked connection with buffered reader/writer
// This ensures that any data buffered by the HTTP server is not lost
type hijackedConn struct {
	net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
}

// Read reads from the buffered reader first, then falls back to the underlying connection
func (c *hijackedConn) Read(p []byte) (n int, err error) {
	// Check if there's buffered data
	if c.reader.Buffered() > 0 {
		return c.reader.Read(p)
	}
	// No buffered data, read directly from connection
	return c.Conn.Read(p)
}

// Write writes through the buffered writer
func (c *hijackedConn) Write(p []byte) (n int, err error) {
	n, err = c.writer.Write(p)
	if err != nil {
		return
	}
	// Flush to ensure data is sent
	err = c.writer.Flush()
	return
}

// WriteTo implements io.WriterTo for efficient copying
func (c *hijackedConn) WriteTo(w io.Writer) (n int64, err error) {
	// First write any buffered data
	if c.reader.Buffered() > 0 {
		buffered := c.reader.Buffered()
		buf := make([]byte, buffered)
		_, err = io.ReadFull(c.reader, buf)
		if err != nil {
			return 0, err
		}
		written, err := w.Write(buf)
		if err != nil {
			return int64(written), err
		}
		n = int64(written)
	}

	// Then copy remaining data from underlying connection
	copied, err := io.Copy(w, c.Conn)
	return n + copied, err
}

// ReadFrom implements io.ReaderFrom for efficient copying
func (c *hijackedConn) ReadFrom(r io.Reader) (n int64, err error) {
	// Flush any buffered writes first
	if c.writer != nil {
		if err := c.writer.Flush(); err != nil {
			return 0, err
		}
	}

	// Copy from reader to underlying connection
	return io.Copy(c.Conn, r)
}
