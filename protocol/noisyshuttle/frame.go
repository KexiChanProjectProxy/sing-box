package noisyshuttle

import (
	"encoding/binary"
	"io"

	E "github.com/sagernet/sing/common/exceptions"
)

type Frame struct {
	Type     byte
	Flags    byte
	StreamID uint32
	Payload  []byte
}

func WriteFrame(writer io.Writer, frameType byte, flags byte, streamID uint32, payload []byte) error {
	if !ValidFrameType(frameType) {
		return E.New("unknown frame type: ", frameType)
	}
	if len(payload) > MaxPayloadLength {
		return E.New("payload too large: ", len(payload))
	}
	var header [HeaderSize]byte
	header[0] = frameType
	header[1] = flags
	binary.BigEndian.PutUint32(header[2:6], streamID)
	binary.BigEndian.PutUint16(header[6:8], uint16(len(payload)))
	if _, err := writer.Write(header[:]); err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil
	}
	_, err := writer.Write(payload)
	return err
}

func ReadFrame(reader io.Reader, maxPayload int) (Frame, error) {
	var header [HeaderSize]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return Frame{}, E.Cause(err, "read frame header")
	}
	frameType := header[0]
	if !ValidFrameType(frameType) {
		return Frame{}, E.New("unknown frame type: ", frameType)
	}
	length := int(binary.BigEndian.Uint16(header[6:8]))
	if maxPayload < 0 || maxPayload > MaxPayloadLength {
		maxPayload = MaxPayloadLength
	}
	if length > maxPayload {
		return Frame{}, E.New("payload too large: ", length)
	}
	payload := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(reader, payload); err != nil {
			return Frame{}, E.Cause(err, "read frame payload")
		}
	}
	frame := Frame{Type: frameType, Flags: header[1], StreamID: binary.BigEndian.Uint32(header[2:6]), Payload: payload}
	if err := ValidateFrameState(frame); err != nil {
		return Frame{}, err
	}
	return frame, nil
}

func ValidateFrameState(frame Frame) error {
	if !ValidFrameType(frame.Type) {
		return E.New("unknown frame type: ", frame.Type)
	}
	control := frame.Type == FrameTypeClientHello || frame.Type == FrameTypeServerHello || frame.Type == FrameTypePing || frame.Type == FrameTypePong || frame.Type == FrameTypeClose
	if control && frame.StreamID != 0 {
		return E.New("control frame with non-zero stream id: ", frame.StreamID)
	}
	if !control && frame.StreamID == 0 {
		return E.New("stream frame with zero stream id")
	}
	return nil
}
