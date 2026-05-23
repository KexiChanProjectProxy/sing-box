package noisyshuttle

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net/netip"
	"testing"
	"time"

	"github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"

	"github.com/stretchr/testify/require"
)

const (
	IntegrationServerPort uint16 = 25201
	IntegrationEchoPort   uint16 = 25202
)

const (
	TestPassword = "test-password"
)

func portStr(p uint16) string {
	return fmt.Sprintf("%d", p)
}

func buildServerOptions(nsPort uint16, password, certPem, keyPem string, sessionOpts option.NoisyShuttleSessionOptions) option.Options {
	return option.Options{
		Inbounds: []option.Inbound{
			{
				Type: constant.TypeNoisyShuttle,
				Tag:  "ns-in",
				Options: &option.NoisyShuttleInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: nsPort,
					},
					Users: []option.NoisyShuttleUser{
						{Name: "testuser", Password: password},
					},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
					},
					Network: "tcp",
					Session: sessionOpts,
					Handshake: option.NoisyShuttleInboundHandshakeOptions{
						MaxPadding:  256,
						AuthTimeout: badoption.Duration(5 * time.Second),
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: constant.TypeDirect,
				Tag:  "direct",
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: constant.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							Inbound: []string{"ns-in"},
						},
						RuleAction: option.RuleAction{
							Action: constant.RuleActionTypeRoute,
							RouteOptions: option.RouteActionOptions{
								Outbound: "direct",
							},
						},
					},
				},
			},
		},
	}
}

func defaultSessionOpts() option.NoisyShuttleSessionOptions {
	return option.NoisyShuttleSessionOptions{
		Enabled:           true,
		MaxStreams:        16,
		MaxRequests:       0,
		IdleTimeout:       badoption.Duration(5 * time.Minute),
		MaxAge:            0,
		KeepaliveInterval: badoption.Duration(30 * time.Second),
	}
}

func TestInstanceCreation(t *testing.T) {
	_, certPem, keyPem := CreateSelfSignedCertificate(t, "example.org")

	serverOpts := buildServerOptions(IntegrationServerPort, TestPassword, certPem, keyPem, defaultSessionOpts())
	serverInstance := StartInstance(t, serverOpts)
	defer serverInstance.Close()

	clientOpts := buildClientOptions(IntegrationClientPort, IntegrationServerPort, TestPassword, certPem, defaultSessionOpts())
	clientInstance := StartInstance(t, clientOpts)
	defer clientInstance.Close()
}

func TestBadAuth(t *testing.T) {
	_, certPem, keyPem := CreateSelfSignedCertificate(t, "example.org")

	serverOpts := buildServerOptions(IntegrationServerPort, TestPassword, certPem, keyPem, defaultSessionOpts())
	serverInstance := StartInstance(t, serverOpts)
	defer serverInstance.Close()

	badClientOpts := buildClientOptions(IntegrationClientPort, IntegrationServerPort, "wrong-password", certPem, defaultSessionOpts())
	badClientInstance := StartInstance(t, badClientOpts)
	defer badClientInstance.Close()
}

type noisyFrame struct {
	Type     byte
	Flags    byte
	StreamID uint32
	Payload  []byte
}

func TestNoisyShuttleProtocolFrame(t *testing.T) {
	_, certPem, keyPem := CreateSelfSignedCertificate(t, "example.org")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_ = ctx

	echoServer, err := NewTCPEchoServer(IntegrationEchoPort)
	require.NoError(t, err)
	defer echoServer.Stop()

	sessionOpts := defaultSessionOpts()
	serverOpts := buildServerOptions(IntegrationServerPort, TestPassword, certPem, keyPem, sessionOpts)
	serverInstance := StartInstance(t, serverOpts)
	defer serverInstance.Close()

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:        "example.org",
	}

	conn, err := tls.Dial("tcp", "127.0.0.1:"+portStr(IntegrationServerPort), tlsConfig)
	require.NoError(t, err)
	defer conn.Close()

	err = conn.Handshake()
	require.NoError(t, err)

	err = sendNoisyShuttlePreface(conn, TestPassword, 16)
	require.NoError(t, err)

	err = sendNoisyShuttleClientHello(conn, 0x03, 16)
	require.NoError(t, err)

	frame, err := readNoisyShuttleFrame(conn)
	require.NoError(t, err)
	require.Equal(t, byte(0x02), frame.Type)

	err = sendNoisyShuttleOpenRequest(conn, 1, "127.0.0.1", IntegrationEchoPort)
	require.NoError(t, err)

	frame, err = readNoisyShuttleFrame(conn)
	require.NoError(t, err)
	require.Equal(t, byte(0x04), frame.Type)
}

const IntegrationClientPort uint16 = 25200

func buildClientOptions(mixedPort, nsPort uint16, password, certPem string, sessionOpts option.NoisyShuttleSessionOptions) option.Options {
	return option.Options{
		Inbounds: []option.Inbound{
			{
				Type: constant.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: mixedPort,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: constant.TypeDirect,
				Tag:  "direct",
			},
			{
				Type: constant.TypeNoisyShuttle,
				Tag:  "ns-out",
				Options: &option.NoisyShuttleOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: nsPort,
					},
					Password: password,
					Network:  "tcp",
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
						TLS: &option.OutboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
						},
					},
					Session: sessionOpts,
					Handshake: option.NoisyShuttleOutboundHandshakeOptions{
						PaddingMin:  16,
						PaddingMax:  256,
						AuthTimeout: badoption.Duration(5 * time.Second),
					},
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: constant.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							Inbound: []string{"mixed-in"},
						},
						RuleAction: option.RuleAction{
							Action: constant.RuleActionTypeRoute,
							RouteOptions: option.RouteActionOptions{
								Outbound: "ns-out",
							},
						},
					},
				},
			},
		},
	}
}

func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}

func sendNoisyShuttlePreface(conn *tls.Conn, password string, paddingLen int) error {
	hash := sha256Hash(password)
	if _, err := conn.Write([]byte(hash)); err != nil {
		return err
	}
	if _, err := conn.Write([]byte{'\r', '\n'}); err != nil {
		return err
	}
	if paddingLen > 0 {
		padding := make([]byte, paddingLen)
		for i := range padding {
			padding[i] = byte(i % 256)
		}
		if _, err := conn.Write(padding); err != nil {
			return err
		}
	}
	_, err := conn.Write([]byte{'\r', '\n'})
	return err
}

func sendNoisyShuttleClientHello(conn *tls.Conn, capabilities uint8, maxStreams uint16) error {
	payload := make([]byte, 6)
	payload[0] = 0x01
	binary.BigEndian.PutUint16(payload[1:3], uint16(capabilities))
	binary.BigEndian.PutUint16(payload[3:5], maxStreams)
	return writeNoisyShuttleFrame(conn, 0x01, 0, 0, payload)
}

func sendNoisyShuttleOpenRequest(conn *tls.Conn, streamID uint32, host string, port uint16) error {
	hostBytes := netParseIPv4(host)
	address := make([]byte, 1+len(hostBytes)+2)
	address[0] = 0x01
	copy(address[1:], hostBytes)
	binary.BigEndian.PutUint16(address[1+len(hostBytes):], port)

	payload := make([]byte, 1+len(address))
	payload[0] = 0x01
	copy(payload[1:], address)
	return writeNoisyShuttleFrame(conn, 0x03, 0, streamID, payload)
}

func writeNoisyShuttleFrame(conn *tls.Conn, frameType, flags byte, streamID uint32, payload []byte) error {
	header := make([]byte, 8)
	header[0] = frameType
	header[1] = flags
	binary.BigEndian.PutUint32(header[2:6], streamID)
	binary.BigEndian.PutUint16(header[6:8], uint16(len(payload)))
	if _, err := conn.Write(header); err != nil {
		return err
	}
	if len(payload) > 0 {
		_, err := conn.Write(payload)
		return err
	}
	return nil
}

func readNoisyShuttleFrame(conn *tls.Conn) (noisyFrame, error) {
	header := make([]byte, 8)
	_, err := io.ReadFull(conn, header)
	if err != nil {
		return noisyFrame{}, err
	}
	frame := noisyFrame{
		Type:     header[0],
		Flags:    header[1],
		StreamID: binary.BigEndian.Uint32(header[2:6]),
	}
	length := binary.BigEndian.Uint16(header[6:8])
	if length > 0 {
		payload := make([]byte, length)
		_, err := io.ReadFull(conn, payload)
		if err != nil {
			return noisyFrame{}, err
		}
		frame.Payload = payload
	}
	return frame, nil
}

func netParseIPv4(host string) []byte {
	if host == "127.0.0.1" {
		return []byte{127, 0, 0, 1}
	}
	return []byte{127, 0, 0, 1}
}
