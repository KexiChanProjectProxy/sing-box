package cloudflarewarp

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
)

const (
	DefaultServerName = "consumer-masque.cloudflareclient.com"
	nextProtoH3       = "h3"
)

type TLSOptions struct {
	// Base supplies server name, clock, roots and version limits. It is cloned.
	Base       *tls.Config
	PrivateKey *ecdsa.PrivateKey
	// EndpointPublicKey pins the server leaf key. Nil falls back to normal
	// certificate verification unless Insecure is set.
	EndpointPublicKey *ecdsa.PublicKey
	Insecure          bool
}

// NewTLSConfig returns a TLS config carrying a fresh self-signed client
// certificate for the enrolled key, as the official client does per connection.
func NewTLSConfig(options TLSOptions) (*tls.Config, error) {
	if options.PrivateKey == nil {
		return nil, E.New("missing private key")
	}
	var config *tls.Config
	if options.Base != nil {
		config = options.Base.Clone()
	} else {
		config = &tls.Config{}
	}
	if config.ServerName == "" {
		config.ServerName = DefaultServerName
	}
	config.NextProtos = []string{nextProtoH3}
	now := time.Now
	if config.Time != nil {
		now = config.Time
	}
	certificate, err := selfSignedCertificate(options.PrivateKey, now())
	if err != nil {
		return nil, err
	}
	config.Certificates = []tls.Certificate{certificate}
	config.GetClientCertificate = nil
	if options.EndpointPublicKey != nil {
		pinnedKey := options.EndpointPublicKey
		config.InsecureSkipVerify = true
		config.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return E.New("server presented no certificate")
			}
			leaf, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return E.Cause(err, "parse server certificate")
			}
			publicKey, isECDSA := leaf.PublicKey.(*ecdsa.PublicKey)
			if !isECDSA || !publicKey.Equal(pinnedKey) {
				return E.New("server certificate does not match endpoint_public_key")
			}
			return nil
		}
	} else if options.Insecure {
		config.InsecureSkipVerify = true
	}
	return config, nil
}

func selfSignedCertificate(key *ecdsa.PrivateKey, now time.Time) (tls.Certificate, error) {
	template := &x509.Certificate{
		SerialNumber: big.NewInt(0),
		NotBefore:    now.Add(-5 * time.Minute),
		NotAfter:     now.Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, E.Cause(err, "create client certificate")
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, nil
}
