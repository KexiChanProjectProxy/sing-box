package noisyshuttle

import (
	"bytes"
	"testing"
)

func TestUDPdatagramRoundTrip(t *testing.T) {
	addr := Address{Host: "example.com"}
	payload := []byte("hello world")
	encoded, err := EncodeUDPdatagram(addr, 443, payload)
	if err != nil {
		t.Fatal(err)
	}
	decodedAddr, port, decodedPayload, err := DecodeUDPdatagram(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decodedAddr.Host != addr.Host || port != 443 || !bytes.Equal(decodedPayload, payload) {
		t.Fatalf("unexpected datagram: addr=%v port=%d payload=%q", decodedAddr, port, decodedPayload)
	}
}

func BenchmarkEncodeUDPdatagram(b *testing.B) {
	addr := Address{Host: "example.com"}
	payload := make([]byte, 512)
	for i := range payload {
		payload[i] = byte(i)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		EncodeUDPdatagram(addr, 443, payload)
	}
}

func BenchmarkDecodeUDPdatagram(b *testing.B) {
	addr := Address{Host: "example.com"}
	payload := make([]byte, 512)
	for i := range payload {
		payload[i] = byte(i)
	}
	encoded, _ := EncodeUDPdatagram(addr, 443, payload)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		DecodeUDPdatagram(encoded)
	}
}
