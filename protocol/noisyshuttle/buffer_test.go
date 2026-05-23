package noisyshuttle

import (
	"bytes"
	"testing"
)

func TestNewFrameBuffer(t *testing.T) {
	buffer, payload, err := NewFrameBuffer(4)
	if err != nil {
		t.Fatal(err)
	}
	defer buffer.Release()
	if len(payload) != 4 || buffer.Len() != HeaderSize+4 {
		t.Fatalf("unexpected buffer length: payload=%d buffer=%d", len(payload), buffer.Len())
	}
}

func TestReadFramePayloadBuffer(t *testing.T) {
	buffer, err := ReadFramePayloadBuffer(bytes.NewReader([]byte("payload")), 7)
	if err != nil {
		t.Fatal(err)
	}
	defer buffer.Release()
	if string(buffer.Bytes()) != "payload" {
		t.Fatalf("unexpected payload: %q", buffer.Bytes())
	}
	if _, err := ReadFramePayloadBuffer(bytes.NewReader([]byte("x")), 2); err == nil {
		t.Fatal("expected short read error")
	}
}
