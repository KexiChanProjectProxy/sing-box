package noisyshuttle

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestPasswordHashHex(t *testing.T) {
	hash := sha256.Sum256(nil)
	expected := hex.EncodeToString(hash[:])
	got := PasswordHashHex("")
	if got != expected {
		t.Fatal("unexpected empty password hash")
	}
}

func TestPrefaceRoundTrip(t *testing.T) {
	var buffer bytes.Buffer
	if err := EncodePreface(&buffer, "secret", 8); err != nil {
		t.Fatal(err)
	}
	hash, padding, err := DecodePreface(&buffer, DefaultServerMaxPadding)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPrefaceHash(hash, "secret") {
		t.Fatal("hash mismatch")
	}
	if len(padding) != 8 {
		t.Fatalf("unexpected padding length: %d", len(padding))
	}
}

func TestDecodePrefaceRejectsBadDelimiter(t *testing.T) {
	hash := PasswordHashHex("secret")
	data := append([]byte(hash), '\n', '\r', '\r', '\n')
	if _, _, err := DecodePreface(bytes.NewReader(data), DefaultServerMaxPadding); err == nil {
		t.Fatal("expected bad delimiter error")
	}
}

func TestDecodePrefaceRejectsOversizedPadding(t *testing.T) {
	hash := PasswordHashHex("secret")
	var data []byte
	data = append(data, hash...)
	data = append(data, '\r', '\n')
	data = append(data, bytes.Repeat([]byte{'x'}, 3)...)
	data = append(data, '\r', '\n')
	if _, _, err := DecodePreface(bytes.NewReader(data), 2); err == nil {
		t.Fatal("expected oversized padding error")
	}
}

func TestDecodePrefaceRejectsEOF(t *testing.T) {
	if _, _, err := DecodePreface(bytes.NewReader([]byte("short")), DefaultServerMaxPadding); err == nil {
		t.Fatal("expected EOF error")
	}
}

func BenchmarkEncodePreface(b *testing.B) {
	var buf bytes.Buffer
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		EncodePreface(&buf, "secretpassword", 16)
	}
}

func BenchmarkDecodePreface(b *testing.B) {
	var buf bytes.Buffer
	EncodePreface(&buf, "secretpassword", 16)
	data := buf.Bytes()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		DecodePreface(bytes.NewReader(data), DefaultServerMaxPadding)
	}
}

func BenchmarkPasswordHash(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		PasswordHashHex("secretpassword")
	}
}
