package noisyshuttle

import "testing"

func TestPingRoundTrip(t *testing.T) {
	ping := Ping{Timestamp: 123456789, Counter: 7, Token: 9}
	decoded, err := DecodePing(EncodePing(ping))
	if err != nil {
		t.Fatal(err)
	}
	if decoded != ping {
		t.Fatalf("unexpected ping: %#v", decoded)
	}
}

func TestPongRoundTripAndValidate(t *testing.T) {
	ping := Ping{Counter: 7, Token: 9}
	pong := Pong{Counter: 7, Token: 9}
	decoded, err := DecodePong(EncodePong(pong))
	if err != nil {
		t.Fatal(err)
	}
	if decoded != pong {
		t.Fatalf("unexpected pong: %#v", decoded)
	}
	if err := ValidatePong(ping, decoded); err != nil {
		t.Fatal(err)
	}
}

func TestKeepaliveValidation(t *testing.T) {
	if _, err := DecodePing([]byte{KeepaliveMagic}); err == nil {
		t.Fatal("expected invalid ping length")
	}
	badPing := EncodePing(Ping{})
	badPing[0] = 0
	if _, err := DecodePing(badPing); err == nil {
		t.Fatal("expected invalid ping magic")
	}
	if _, err := DecodePong([]byte{KeepaliveMagic}); err == nil {
		t.Fatal("expected invalid pong length")
	}
	badPong := EncodePong(Pong{})
	badPong[0] = 0
	if _, err := DecodePong(badPong); err == nil {
		t.Fatal("expected invalid pong magic")
	}
	if err := ValidatePong(Ping{Counter: 1, Token: 2}, Pong{Counter: 1, Token: 3}); err == nil {
		t.Fatal("expected pong mismatch")
	}
}

func BenchmarkEncodePing(b *testing.B) {
	ping := Ping{Timestamp: 1234567890, Counter: 7, Token: 9}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		EncodePing(ping)
	}
}

func BenchmarkDecodePing(b *testing.B) {
	data := EncodePing(Ping{Timestamp: 1234567890, Counter: 7, Token: 9})
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		DecodePing(data)
	}
}

func BenchmarkEncodePong(b *testing.B) {
	pong := Pong{Counter: 7, Token: 9}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		EncodePong(pong)
	}
}
