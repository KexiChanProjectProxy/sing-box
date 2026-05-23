package noisyshuttle

import (
	"encoding/binary"
	"io"

	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
)

func NewPayloadBuffer(size int) *buf.Buffer {
	return buf.NewSize(size)
}

func NewFrameBuffer(payloadLen int) (*buf.Buffer, []byte, error) {
	if payloadLen < 0 || payloadLen > MaxPayloadLength {
		return nil, nil, E.New("payload too large: ", payloadLen)
	}
	buffer := buf.NewSize(HeaderSize + payloadLen)
	payload := buffer.Extend(HeaderSize + payloadLen)[HeaderSize:]
	return buffer, payload, nil
}

func WriteBufferedFrame(writer io.Writer, frameType byte, flags byte, streamID uint32, payload []byte) error {
	if !ValidFrameType(frameType) {
		return E.New("unknown frame type: ", frameType)
	}
	buffer, framePayload, err := NewFrameBuffer(len(payload))
	if err != nil {
		return err
	}
	defer buffer.Release()
	frame := buffer.Bytes()
	frame[0] = frameType
	frame[1] = flags
	binary.BigEndian.PutUint32(frame[2:6], streamID)
	binary.BigEndian.PutUint16(frame[6:8], uint16(len(payload)))
	copy(framePayload, payload)
	_, err = buffer.WriteTo(writer)
	return err
}

func ReadFramePayloadBuffer(reader io.Reader, length int) (*buf.Buffer, error) {
	if length < 0 || length > MaxPayloadLength {
		return nil, E.New("payload too large: ", length)
	}
	buffer := buf.NewSize(length)
	if length == 0 {
		return buffer, nil
	}
	if _, err := buffer.ReadFullFrom(reader, length); err != nil {
		buffer.Release()
		return nil, err
	}
	return buffer, nil
}
