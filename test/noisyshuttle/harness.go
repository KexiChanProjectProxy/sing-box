package noisyshuttle

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/control"
	"github.com/sagernet/sing/common/debug"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

var globalCtx = context.Background()

func init() {
	globalCtx = include.Context(context.Background())
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

const (
	HarnessClientPort uint16 = 15000 + iota
	HarnessServerPort
	HarnessTestPort
	HarnessOutPort
)

type TCPEchoServer struct {
	addr   net.Addr
	l      net.Listener
	stopCh chan struct{}
	wg     sync.WaitGroup
}

func NewTCPEchoServer(port uint16) (*TCPEchoServer, error) {
	addr := net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(port)}
	l, err := net.ListenTCP("tcp", &addr)
	if err != nil {
		return nil, err
	}
	s := &TCPEchoServer{
		addr:   l.Addr(),
		l:      l,
		stopCh: make(chan struct{}),
	}
	s.wg.Add(1)
	go s.serve()
	return s, nil
}

func (s *TCPEchoServer) serve() {
	defer s.wg.Done()
	for {
		conn, err := s.l.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				return
			}
		}
		go s.handleConn(conn)
	}
}

func (s *TCPEchoServer) handleConn(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 65536)
	for {
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		_, err = conn.Write(buf[:n])
		if err != nil {
			return
		}
	}
}

func (s *TCPEchoServer) Addr() net.Addr {
	return s.addr
}

func (s *TCPEchoServer) Stop() {
	close(s.stopCh)
	s.l.Close()
	s.wg.Wait()
}

type UDPEchoServer struct {
	addr   *net.UDPAddr
	conn   *net.UDPConn
	stopCh chan struct{}
	wg     sync.WaitGroup
}

func NewUDPEchoServer(port uint16) (*UDPEchoServer, error) {
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(port)}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	s := &UDPEchoServer{
		addr:   addr,
		conn:   conn,
		stopCh: make(chan struct{}),
	}
	s.wg.Add(1)
	go s.serve()
	return s, nil
}

func (s *UDPEchoServer) serve() {
	defer s.wg.Done()
	buf := make([]byte, 65536)
	for {
		s.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, addr, err := s.conn.ReadFrom(buf)
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				return
			}
		}
		_, err = s.conn.WriteTo(buf[:n], addr)
		if err != nil {
			return
		}
	}
}

func (s *UDPEchoServer) Addr() *net.UDPAddr {
	return s.addr
}

func (s *UDPEchoServer) Stop() {
	close(s.stopCh)
	s.conn.Close()
	s.wg.Wait()
}

type InstanceHelper struct {
	Box    *box.Box
	Cancel context.CancelFunc
}

func StartInstance(t *testing.T, options option.Options) *InstanceHelper {
	if debug.Enabled {
		options.Log = &option.LogOptions{
			Level: "trace",
		}
	} else {
		options.Log = &option.LogOptions{
			Level: "warning",
		}
	}
	ctx, cancel := context.WithCancel(globalCtx)
	var instance *box.Box
	var err error
	for retry := 0; retry < 3; retry++ {
		instance, err = box.New(box.Options{
			Context: ctx,
			Options: options,
		})
		require.NoError(t, err)
		err = instance.Start()
		if err != nil {
			time.Sleep(time.Second)
			cancel()
			ctx, cancel = context.WithCancel(globalCtx)
			continue
		}
		break
	}
	require.NoError(t, err)
	t.Cleanup(func() {
		instance.Close()
		cancel()
	})
	return &InstanceHelper{
		Box:    instance,
		Cancel: cancel,
	}
}

func (h *InstanceHelper) Close() {
	if h.Box != nil {
		h.Box.Close()
	}
	if h.Cancel != nil {
		h.Cancel()
	}
}

func DialTCP(ctx context.Context, addr string) (net.Conn, error) {
	return net.Dial("tcp", addr)
}

func DialUDP(ctx context.Context, addr string) (net.PacketConn, error) {
	return ListenPacket("udp", ":0")
}

func PingPong(t *testing.T, conn net.Conn, timeout time.Duration) error {
	conn.SetWriteDeadline(time.Now().Add(timeout))
	_, err := conn.Write([]byte("ping"))
	if err != nil {
		return err
	}
	conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 4)
	_, err = io.ReadFull(conn, buf)
	if err != nil {
		return err
	}
	if string(buf) != "pong" {
		return err
	}
	return nil
}

func PingPongUDP(t *testing.T, pc net.PacketConn, addr net.Addr, timeout time.Duration) error {
	pc.SetWriteDeadline(time.Now().Add(timeout))
	_, err := pc.WriteTo([]byte("ping"), addr)
	if err != nil {
		return err
	}
	pc.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 1024)
	n, _, err := pc.ReadFrom(buf)
	if err != nil {
		return err
	}
	if string(buf[:n]) != "ping" {
		return err
	}
	return nil
}

func WithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}

func WithRetry(maxRetries int, delay time.Duration, fn func() error) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		time.Sleep(delay)
	}
	return err
}

func CreateSelfSignedCertificate(t *testing.T, domain string) (caPem, certPem, keyPem string) {
	const userAndHostname = "noisyshuttle-test@localhost"
	tempDir, err := os.MkdirTemp("", "noisyshuttle-test")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	caKey, err := rsa.GenerateKey(rand.Reader, 3072)
	require.NoError(t, err)

	spkiASN1, err := x509.MarshalPKIXPublicKey(caKey.Public())
	var spki struct {
		Algorithm        pkix.AlgorithmIdentifier
		SubjectPublicKey asn1.BitString
	}
	_, err = asn1.Unmarshal(spkiASN1, &spki)
	require.NoError(t, err)
	skid := sha1.Sum(spki.SubjectPublicKey.Bytes)

	caTpl := &x509.Certificate{
		SerialNumber:          randomSerialNumber(t),
		Subject:               pkix.Name{Organization: []string{"noisyshuttle test CA"}, OrganizationalUnit: []string{userAndHostname}, CommonName: "noisyshuttle " + userAndHostname},
		SubjectKeyId:          skid[:],
		NotAfter:              time.Now().AddDate(10, 0, 0),
		NotBefore:             time.Now(),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	caCert, err := x509.CreateCertificate(rand.Reader, caTpl, caTpl, caKey.Public(), caKey)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "ca.pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert}), 0o600)
	require.NoError(t, err)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	domainTpl := &x509.Certificate{
		SerialNumber: randomSerialNumber(t),
		Subject:      pkix.Name{Organization: []string{"noisyshuttle test certificate"}, OrganizationalUnit: []string{"noisyshuttle " + userAndHostname}},
		NotBefore:    time.Now(), NotAfter: time.Now().AddDate(0, 0, 30),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	domainTpl.DNSNames = append(domainTpl.DNSNames, domain)
	cert, err := x509.CreateCertificate(rand.Reader, domainTpl, caTpl, key.Public(), caKey)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})
	privDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})
	err = os.WriteFile(filepath.Join(tempDir, domain+".pem"), certPEM, 0o600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, domain+".key"), privPEM, 0o600)
	require.NoError(t, err)

	return filepath.Join(tempDir, "ca.pem"), filepath.Join(tempDir, domain+".pem"), filepath.Join(tempDir, domain+".key")
}

func randomSerialNumber(t *testing.T) *big.Int {
	serial := make([]byte, 16)
	_, err := rand.Read(serial)
	require.NoError(t, err)
	return new(big.Int).SetBytes(serial)
}

func ListenPacket(network, address string) (net.PacketConn, error) {
	var lc net.ListenConfig
	lc.Control = control.ReuseAddr()
	var lastErr error
	for i := 0; i < 5; i++ {
		l, err := lc.ListenPacket(context.Background(), network, address)
		if err == nil {
			return l, nil
		}
		lastErr = err
		time.Sleep(5 * time.Millisecond)
	}
	return nil, lastErr
}

func ListenTCP(address string) (net.Listener, error) {
	var lc net.ListenConfig
	lc.Control = control.ReuseAddr()
	var lastErr error
	for i := 0; i < 5; i++ {
		l, err := lc.Listen(context.Background(), "tcp", address)
		if err == nil {
			return l, nil
		}
		lastErr = err
		time.Sleep(5 * time.Millisecond)
	}
	return nil, lastErr
}

func LoopbackIP() netip.Addr {
	return netip.MustParseAddr("127.0.0.1")
}
