package tf_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"

	tf "github.com/sagernet/sing-box/common/tlsfragment"

	"github.com/stretchr/testify/require"
)

func newLocalTLSServer() (net.Listener, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, err
	}

	cert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  privateKey,
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	return tls.NewListener(lis, &tls.Config{Certificates: []tls.Certificate{cert}}), nil
}

func runEchoServer(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			return
		}
		go func() {
			buf := make([]byte, 1024)
			conn.Read(buf)
			conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
			conn.Close()
		}()
	}
}

func TestTLSFragment(t *testing.T) {
	t.Parallel()

	lis, err := newLocalTLSServer()
	require.NoError(t, err)
	defer lis.Close()

	go runEchoServer(lis)

	tcpConn, err := net.Dial("tcp", lis.Addr().String())
	require.NoError(t, err)

	fragConn := tf.NewConn(tcpConn, context.Background(), true, false, 0)
	tlsConn := tls.Client(fragConn, &tls.Config{
		ServerName:         "localhost",
		InsecureSkipVerify: true,
	})
	require.NoError(t, tlsConn.Handshake())
	tlsConn.Close()
}

func TestTLSRecordFragment(t *testing.T) {
	t.Parallel()

	lis, err := newLocalTLSServer()
	require.NoError(t, err)
	defer lis.Close()

	go runEchoServer(lis)

	tcpConn, err := net.Dial("tcp", lis.Addr().String())
	require.NoError(t, err)

	fragConn := tf.NewConn(tcpConn, context.Background(), false, true, 0)
	tlsConn := tls.Client(fragConn, &tls.Config{
		ServerName:         "localhost",
		InsecureSkipVerify: true,
	})
	require.NoError(t, tlsConn.Handshake())
	tlsConn.Close()
}

func TestTLS2Fragment(t *testing.T) {
	t.Parallel()

	lis, err := newLocalTLSServer()
	require.NoError(t, err)
	defer lis.Close()

	go runEchoServer(lis)

	tcpConn, err := net.Dial("tcp", lis.Addr().String())
	require.NoError(t, err)

	fragConn := tf.NewConn(tcpConn, context.Background(), true, true, 0)
	tlsConn := tls.Client(fragConn, &tls.Config{
		ServerName:         "localhost",
		InsecureSkipVerify: true,
	})
	require.NoError(t, tlsConn.Handshake())
	tlsConn.Close()
}