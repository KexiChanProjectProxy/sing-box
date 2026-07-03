package firefoxvpn

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeHTTPSServer struct {
	URL       string
	rootCAPEM []byte
	server    *httptest.Server
}

func (s *fakeHTTPSServer) port() uint16 {
	return uint16(s.server.Listener.Addr().(*net.TCPAddr).Port)
}

func newFakeHTTPSServer(t *testing.T, serverName string, handler http.Handler) *fakeHTTPSServer {
	t.Helper()
	certificate, certPEM := mustSelfSignedCertificate(t, serverName)
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{certificate}}
	server.StartTLS()
	t.Cleanup(server.Close)
	return &fakeHTTPSServer{URL: server.URL, rootCAPEM: certPEM, server: server}
}

func newFakeHTTP2Server(t *testing.T, serverName string, handler http.Handler) *fakeHTTPSServer {
	t.Helper()
	certificate, certPEM := mustSelfSignedCertificate(t, serverName)
	server := httptest.NewUnstartedServer(handler)
	server.EnableHTTP2 = true
	server.TLS = &tls.Config{Certificates: []tls.Certificate{certificate}, NextProtos: []string{"h2"}}
	server.StartTLS()
	t.Cleanup(server.Close)
	return &fakeHTTPSServer{URL: server.URL, rootCAPEM: certPEM, server: server}
}

func mustSelfSignedCertificate(t *testing.T, serverName string) (tls.Certificate, []byte) {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{CommonName: serverName},
		DNSNames:     []string{serverName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, privateKey.Public(), privateKey)
	require.NoError(t, err)
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER})
	certificate, err := tls.X509KeyPair(certificatePEM, privateKeyPEM)
	require.NoError(t, err)
	return certificate, certificatePEM
}

type flushWriter struct {
	Writer  io.Writer
	Flusher http.Flusher
}

func (w flushWriter) Write(buffer []byte) (int, error) {
	n, err := w.Writer.Write(buffer)
	if err == nil && w.Flusher != nil {
		w.Flusher.Flush()
	}
	return n, err
}

func combineRootCAs(pems ...[]byte) []byte {
	parts := make([]string, 0, len(pems))
	for _, certPEM := range pems {
		parts = append(parts, string(certPEM))
	}
	return []byte(strings.Join(parts, "\n"))
}

func mustWriteCertBundle(t *testing.T, certBundle []byte) string {
	t.Helper()
	bundleFile := t.TempDir() + "/roots.pem"
	require.NoError(t, os.WriteFile(bundleFile, certBundle, 0o600))
	return bundleFile
}
