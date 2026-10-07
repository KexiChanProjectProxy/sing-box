package warpapi

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/netip"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"
)

// GeneratePrivateKey creates the ECDSA P-256 key WARP enrolls for MASQUE.
func GeneratePrivateKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

// EncodePrivateKey encodes the key as base64 SEC 1 DER, the format usque stores.
func EncodePrivateKey(key *ecdsa.PrivateKey) (string, error) {
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(der), nil
}

// ParsePrivateKey accepts base64 SEC 1 or PKCS #8 DER, or a PEM block of either.
func ParsePrivateKey(content string) (*ecdsa.PrivateKey, error) {
	der, err := decodeKeyContent(content)
	if err != nil {
		return nil, E.Cause(err, "decode private key")
	}
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		return checkCurve(key)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, E.New("parse private key: expected an ECDSA P-256 key in SEC 1 or PKCS #8 form")
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, E.New("parse private key: not an ECDSA key")
	}
	return checkCurve(key)
}

func checkCurve(key *ecdsa.PrivateKey) (*ecdsa.PrivateKey, error) {
	if key.Curve != elliptic.P256() {
		return nil, E.New("parse private key: curve must be P-256")
	}
	return key, nil
}

// EncodePublicKey encodes a public key as base64 PKIX DER.
func EncodePublicKey(key *ecdsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(der), nil
}

// ParsePublicKey accepts base64 PKIX DER or a PEM "PUBLIC KEY" block.
func ParsePublicKey(content string) (*ecdsa.PublicKey, error) {
	der, err := decodeKeyContent(content)
	if err != nil {
		return nil, E.Cause(err, "decode public key")
	}
	parsed, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, E.Cause(err, "parse public key")
	}
	key, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return nil, E.New("parse public key: not an ECDSA key")
	}
	return key, nil
}

// NormalizePublicKey converts PEM or base64 DER input to base64 PKIX DER.
func NormalizePublicKey(content string) (string, error) {
	key, err := ParsePublicKey(content)
	if err != nil {
		return "", err
	}
	return EncodePublicKey(key)
}

func decodeKeyContent(content string) ([]byte, error) {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "-----BEGIN") {
		block, _ := pem.Decode([]byte(content))
		if block == nil {
			return nil, E.New("invalid PEM block")
		}
		return block.Bytes, nil
	}
	return base64.StdEncoding.DecodeString(content)
}

// NormalizeEndpoint parses API endpoint values such as "162.159.198.1:0" or
// "[2606:4700:103::]:0" into a bare address.
func NormalizeEndpoint(endpoint string) (netip.Addr, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return netip.Addr{}, nil
	}
	if addrPort, err := netip.ParseAddrPort(endpoint); err == nil {
		return addrPort.Addr().Unmap(), nil
	}
	addr, err := netip.ParseAddr(strings.Trim(endpoint, "[]"))
	if err != nil {
		return netip.Addr{}, E.New("invalid endpoint address: ", endpoint)
	}
	return addr.Unmap(), nil
}
