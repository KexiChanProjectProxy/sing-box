package noisyshuttle

import (
	"bytes"
	"io"
	"net"
	"sync"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
)

type streamConn struct {
	net.Conn
	streamID     uint32
	firstPayload []byte
	readBuffer   bytes.Buffer
	writeMux     sync.Mutex
	readEOF      bool
	session      *clientSession
	inbound      *inboundSession
	released     bool
}

func (c *streamConn) Read(p []byte) (int, error) {
	if len(c.firstPayload) > 0 {
		n := copy(p, c.firstPayload)
		c.firstPayload = c.firstPayload[n:]
		return n, nil
	}
	if c.readBuffer.Len() > 0 {
		return c.readBuffer.Read(p)
	}
	if c.readEOF {
		return 0, io.EOF
	}
	for {
		frame, err := c.readFrame()
		if err != nil {
			return 0, err
		}
		if frame.StreamID != c.streamID {
			return 0, E.New("unexpected stream id: ", frame.StreamID)
		}
		switch frame.Type {
		case FrameTypeData:
			if len(frame.Payload) == 0 {
				continue
			}
			c.readBuffer.Write(frame.Payload)
			return c.readBuffer.Read(p)
		case FrameTypeEndRequest, FrameTypeEndResponse, FrameTypeClose:
			c.readEOF = true
			return 0, io.EOF
		case FrameTypeReset:
			return 0, E.New("stream reset")
		case FrameTypePing:
			_ = c.writeFrame(FrameTypePong, 0, 0, frame.Payload)
		case FrameTypePong:
			continue
		default:
			return 0, E.New("unexpected stream frame type: ", frame.Type)
		}
	}
}

func (c *streamConn) Write(p []byte) (int, error) {
	c.writeMux.Lock()
	defer c.writeMux.Unlock()
	written := 0
	for written < len(p) {
		end := written + MaxPayloadLength
		if end > len(p) {
			end = len(p)
		}
		if err := c.writeFrame(FrameTypeData, 0, c.streamID, p[written:end]); err != nil {
			return written, err
		}
		written = end
	}
	return written, nil
}

func (c *streamConn) Close() error {
	c.writeMux.Lock()
	_ = c.writeFrame(FrameTypeEndResponse, 0, c.streamID, nil)
	c.writeMux.Unlock()
	if c.session != nil {
		_ = c.Conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		_, _ = c.readFrame()
		_ = c.Conn.SetReadDeadline(time.Time{})
		c.release(true)
		return nil
	}
	if c.inbound != nil {
		c.release(true)
		return nil
	}
	return c.Conn.Close()
}

func (c *streamConn) SetDeadline(t time.Time) error {
	return c.Conn.SetDeadline(t)
}

func (c *streamConn) readFrame() (Frame, error) {
	if c.session != nil {
		return readFrameWithSessionActivity(c.Conn, MaxPayloadLength, c.session.markActivity)
	}
	if c.inbound != nil {
		return readFrameWithSessionActivity(c.Conn, MaxPayloadLength, c.inbound.markActivity)
	}
	return ReadFrame(c.Conn, MaxPayloadLength)
}

func (c *streamConn) writeFrame(frameType byte, flags byte, streamID uint32, payload []byte) error {
	if c.session != nil {
		return c.session.writeFrame(frameType, flags, streamID, payload)
	}
	if c.inbound != nil {
		return c.inbound.writeFrame(frameType, flags, streamID, payload)
	}
	return WriteBufferedFrame(c.Conn, frameType, flags, streamID, payload)
}

func (c *streamConn) release(reusable bool) {
	if c.released {
		return
	}
	c.released = true
	if c.session != nil {
		c.session.release(reusable)
	}
	if c.inbound != nil {
		c.inbound.release()
	}
}
