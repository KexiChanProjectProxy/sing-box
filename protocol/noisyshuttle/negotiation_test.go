package noisyshuttle

import "testing"

func TestHelloRoundTrip(t *testing.T) {
	hello := Hello{Version: ProtocolVersion, Capabilities: CapabilityReuse | CapabilityKeepalive, MaxStreams: 16, Reserved: 9}
	decoded, err := DecodeHello(EncodeHello(hello))
	if err != nil {
		t.Fatal(err)
	}
	if decoded != hello {
		t.Fatalf("unexpected hello: %#v", decoded)
	}
}

func TestHelloRejectsInvalidLength(t *testing.T) {
	if _, err := DecodeHello([]byte{1}); err == nil {
		t.Fatal("expected invalid length error")
	}
}

func TestHelloVersionValidation(t *testing.T) {
	if err := ValidateClientHello(Hello{Version: ProtocolVersion + 1}); err == nil {
		t.Fatal("expected client version mismatch")
	}
	if err := ValidateServerHello(Hello{Version: ProtocolVersion + 1}); err == nil {
		t.Fatal("expected server version mismatch")
	}
}
