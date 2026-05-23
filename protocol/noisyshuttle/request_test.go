package noisyshuttle

import (
	"bytes"
	"testing"
)

func TestAddressRoundTrip(t *testing.T) {
	cases := []Address{{Host: "127.0.0.1"}, {Host: "example.com"}, {Host: "2001:db8::1"}}
	for _, testCase := range cases {
		encoded, err := EncodeAddress(testCase)
		if err != nil {
			t.Fatal(err)
		}
		decoded, offset, err := DecodeAddress(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if offset != len(encoded) || decoded.Host != testCase.Host {
			t.Fatalf("unexpected address: %#v offset=%d", decoded, offset)
		}
	}
}

func TestAddressRejectsInvalid(t *testing.T) {
	if _, err := EncodeAddress(Address{Type: AddressTypeDomain}); err == nil {
		t.Fatal("expected empty domain error")
	}
	if _, err := EncodeAddress(Address{Type: AddressTypeDomain, Host: string(bytes.Repeat([]byte{'x'}, 256))}); err == nil {
		t.Fatal("expected long domain error")
	}
	if _, _, err := DecodeAddress([]byte{AddressTypeIPv4, 1}); err == nil {
		t.Fatal("expected truncated ipv4 error")
	}
	if _, _, err := DecodeAddress([]byte{AddressTypeDomain, 0}); err == nil {
		t.Fatal("expected empty domain error")
	}
	if _, _, err := DecodeAddress([]byte{0xff}); err == nil {
		t.Fatal("expected invalid address type error")
	}
}

func TestOpenRequestRoundTrip(t *testing.T) {
	request := OpenRequest{Command: CommandConnect, Address: Address{Host: "example.com"}, Port: 443}
	payload, err := EncodeOpenRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeOpenRequest(payload)
	if err != nil {
		t.Fatal(err)
	}
	request.Address.Type = AddressTypeDomain
	if decoded != request {
		t.Fatalf("unexpected request: %#v", decoded)
	}
}

func TestOpenRequestValidation(t *testing.T) {
	if _, err := EncodeOpenRequest(OpenRequest{Command: 2, Address: Address{Host: "example.com"}, Port: 1}); err == nil {
		t.Fatal("expected unsupported command")
	}
	if _, err := EncodeOpenRequest(OpenRequest{Command: CommandConnect, Address: Address{Host: "example.com"}}); err == nil {
		t.Fatal("expected invalid port")
	}
	if _, err := DecodeOpenRequest([]byte{CommandConnect, AddressTypeDomain, 1, 'a', 0, 1, 0}); err == nil {
		t.Fatal("expected invalid length")
	}
}

func TestOpenResponseRoundTrip(t *testing.T) {
	response := OpenResponse{Status: ErrorDialFailed, Message: "dial failed"}
	payload, err := EncodeOpenResponse(response)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeOpenResponse(payload)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != response {
		t.Fatalf("unexpected response: %#v", decoded)
	}
}

func TestOpenResponseValidation(t *testing.T) {
	if _, err := EncodeOpenResponse(OpenResponse{Message: string(bytes.Repeat([]byte{'x'}, 256))}); err == nil {
		t.Fatal("expected long message error")
	}
	if _, err := DecodeOpenResponse([]byte{0, 0}); err == nil {
		t.Fatal("expected truncated response")
	}
	if _, err := DecodeOpenResponse([]byte{0, 0, 2, 'x'}); err == nil {
		t.Fatal("expected invalid response length")
	}
}

func BenchmarkEncodeAddress(b *testing.B) {
	cases := []Address{
		{Host: "127.0.0.1"},
		{Host: "example.com"},
		{Host: "2001:db8::1"},
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		EncodeAddress(cases[i%len(cases)])
	}
}

func BenchmarkDecodeAddress(b *testing.B) {
	encoded, _ := EncodeAddress(Address{Host: "example.com"})
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		DecodeAddress(encoded)
	}
}
