package noisyshuttle

import (
	"bytes"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	var buffer bytes.Buffer
	payload := []byte("hello")
	if err := WriteFrame(&buffer, FrameTypeData, 0x80, 7, payload); err != nil {
		t.Fatal(err)
	}
	frame, err := ReadFrame(&buffer, MaxPayloadLength)
	if err != nil {
		t.Fatal(err)
	}
	if frame.Type != FrameTypeData || frame.Flags != 0x80 || frame.StreamID != 7 || !bytes.Equal(frame.Payload, payload) {
		t.Fatalf("unexpected frame: %#v", frame)
	}
}

func TestFrameRejectsUnknownType(t *testing.T) {
	data := []byte{0xff, 0, 0, 0, 0, 0, 0, 0}
	if _, err := ReadFrame(bytes.NewReader(data), MaxPayloadLength); err == nil {
		t.Fatal("expected unknown frame error")
	}
	if err := WriteFrame(bytes.NewBuffer(nil), 0xff, 0, 0, nil); err == nil {
		t.Fatal("expected unknown frame write error")
	}
}

func TestFrameRejectsOversizedPayload(t *testing.T) {
	var buffer bytes.Buffer
	if err := WriteFrame(&buffer, FrameTypeClose, 0, 0, bytes.Repeat([]byte{'x'}, 4)); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFrame(&buffer, 3); err == nil {
		t.Fatal("expected oversized payload error")
	}
	if err := WriteFrame(bytes.NewBuffer(nil), FrameTypeClose, 0, 0, bytes.Repeat([]byte{'x'}, MaxPayloadLength+1)); err == nil {
		t.Fatal("expected oversized write error")
	}
}

func TestFrameRejectsTruncatedHeader(t *testing.T) {
	if _, err := ReadFrame(bytes.NewReader([]byte{FrameTypeClose}), MaxPayloadLength); err == nil {
		t.Fatal("expected truncated header error")
	}
}

func TestFrameRejectsTruncatedPayload(t *testing.T) {
	data := []byte{FrameTypeClose, 0, 0, 0, 0, 0, 0, 2, 'x'}
	if _, err := ReadFrame(bytes.NewReader(data), MaxPayloadLength); err == nil {
		t.Fatal("expected truncated payload error")
	}
}

func TestFrameRejectsInvalidState(t *testing.T) {
	var control bytes.Buffer
	if err := WriteFrame(&control, FrameTypeClientHello, 0, 1, EncodeHello(Hello{Version: ProtocolVersion})); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFrame(&control, MaxPayloadLength); err == nil {
		t.Fatal("expected invalid control stream id error")
	}

	var stream bytes.Buffer
	if err := WriteFrame(&stream, FrameTypeData, 0, 0, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFrame(&stream, MaxPayloadLength); err == nil {
		t.Fatal("expected invalid stream id error")
	}
}

func TestBufferedFrameRoundTrip(t *testing.T) {
	var buffer bytes.Buffer
	if err := WriteBufferedFrame(&buffer, FrameTypeOpenResponse, 0, 9, []byte{0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	frame, err := ReadFrame(&buffer, MaxPayloadLength)
	if err != nil {
		t.Fatal(err)
	}
	if frame.Type != FrameTypeOpenResponse || frame.StreamID != 9 || !bytes.Equal(frame.Payload, []byte{0, 0, 0}) {
		t.Fatalf("unexpected frame: %#v", frame)
	}
}

func BenchmarkWriteFrame(b *testing.B) {
	var buf bytes.Buffer
	payload := make([]byte, 256)
	for i := range payload {
		payload[i] = byte(i)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		WriteFrame(&buf, FrameTypeData, 0x80, 7, payload)
	}
}

func BenchmarkReadFrame(b *testing.B) {
	var buf bytes.Buffer
	payload := make([]byte, 256)
	for i := range payload {
		payload[i] = byte(i)
	}
	WriteFrame(&buf, FrameTypeData, 0x80, 7, payload)
	data := buf.Bytes()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ReadFrame(bytes.NewReader(data), MaxPayloadLength)
	}
}

func BenchmarkWriteBufferedFrame(b *testing.B) {
	var buf bytes.Buffer
	payload := make([]byte, 256)
	for i := range payload {
		payload[i] = byte(i)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		WriteBufferedFrame(&buf, FrameTypeData, 0x80, 7, payload)
	}
}
