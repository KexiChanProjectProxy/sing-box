package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"testing"
	"time"

	"github.com/sagernet/sing-box"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	hysteria2protocol "github.com/sagernet/sing-box/protocol/hysteria2"
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/stretchr/testify/require"
)

type recordingRealmDialer struct {
	attempts chan netip.Addr
}

func newRecordingRealmDialer() *recordingRealmDialer {
	return &recordingRealmDialer{attempts: make(chan netip.Addr, 8)}
}

func (d *recordingRealmDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return nil, context.Canceled
}

func (d *recordingRealmDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	select {
	case d.attempts <- destination.Addr:
	default:
	}
	return nil, context.Canceled
}

func startTestRealmSTUNServer(t *testing.T) []string {
	t.Helper()
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	go func() {
		buffer := make([]byte, 1500)
		for {
			n, address, readErr := conn.ReadFrom(buffer)
			if readErr != nil {
				return
			}
			response, ok := buildTestSTUNResponse(buffer[:n], address)
			if !ok {
				continue
			}
			_, _ = conn.WriteTo(response, address)
		}
	}()
	return []string{conn.LocalAddr().String()}
}

func buildTestSTUNResponse(request []byte, address net.Addr) ([]byte, bool) {
	udpAddress, ok := address.(*net.UDPAddr)
	if !ok || len(request) < 20 || binary.BigEndian.Uint32(request[4:8]) != 0x2112A442 {
		return nil, false
	}
	ip4 := udpAddress.IP.To4()
	if ip4 == nil {
		return nil, false
	}
	response := make([]byte, 32)
	binary.BigEndian.PutUint16(response[0:2], 0x0101)
	binary.BigEndian.PutUint16(response[2:4], 12)
	copy(response[4:20], request[4:20])
	binary.BigEndian.PutUint16(response[20:22], 0x0020)
	binary.BigEndian.PutUint16(response[22:24], 8)
	response[25] = 0x01
	binary.BigEndian.PutUint16(response[26:28], uint16(udpAddress.Port)^0x2112)
	for i := range ip4 {
		response[28+i] = ip4[i] ^ request[4+i]
	}
	return response, true
}

func testRealmControlServiceOptions(port uint16, token string) []option.Service {
	return []option.Service{{
		Type: C.TypeHysteriaRealm,
		Options: &option.HysteriaRealmServiceOptions{
			ListenOptions: option.ListenOptions{
				Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
				ListenPort: port,
			},
			Users: []option.HysteriaRealmUser{{
				Name:  "test-user",
				Token: token,
			}},
		},
	}}
}

func startTestRealmControlService(t *testing.T, port uint16, token string) *box.Box {
	t.Helper()
	return startInstance(t, option.Options{
		Services: testRealmControlServiceOptions(port, token),
	})
}

func TestHysteria2Realm_basicLifeCycle(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")

	instance := startInstance(t, option.Options{
		Services: []option.Service{
			{
				Type: C.TypeHysteriaRealm,
				Options: &option.HysteriaRealmServiceOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
					},
					Users: []option.HysteriaRealmUser{{
						Name:  "test-user",
						Token: "test-realm-token",
					}},
				},
			},
		},
	})

	require.NotNil(t, instance, "instance should start when realm is configured")

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, F.ToString("127.0.0.1:", serverPort))
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	controlPlaneURL := F.ToString("https://127.0.0.1:", serverPort, "/v1/test-realm-1/")
	req, err := http.NewRequest(http.MethodPost, controlPlaneURL, bytes.NewBufferString(`{"addresses":["127.0.0.1:12345"]}`))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer test-realm-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	require.NoError(t, err, "realm control plane should be accessible at /v1/{realm_id}/")
	require.NotNil(t, resp, "realm control plane should return a response")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestHysteria2Realm_TokenRejection(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")

	instance := startInstance(t, option.Options{
		Services: []option.Service{
			{
				Type: C.TypeHysteriaRealm,
				Options: &option.HysteriaRealmServiceOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
					},
					Users: []option.HysteriaRealmUser{{
						Name:  "test-user",
						Token: "test-realm-token",
					}},
				},
			},
		},
	})
	require.NotNil(t, instance, "instance should start when realm service is configured")

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, F.ToString("127.0.0.1:", serverPort))
			},
		},
	}
	controlPlaneURL := F.ToString("https://127.0.0.1:", serverPort, "/v1/test-realm-token/")
	req, err := http.NewRequest(http.MethodPost, controlPlaneURL, bytes.NewBufferString(`{"addresses":["127.0.0.1:12345"]}`))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer invalid-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestHysteria2Realm_PortRangeSequentialBind(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	stunServers := startTestRealmSTUNServer(t)
	startTestRealmControlService(t, serverPort, "test-token")

	instance := startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeHysteria2,
				Options: &option.Hysteria2InboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					UpMbps:   100,
					DownMbps: 100,
					Users: []option.Hysteria2User{{
						Password: "password",
					}},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
					},
					Realm: &option.Hysteria2InboundRealm{
						Hysteria2Realm: option.Hysteria2Realm{
							ServerURL:   F.ToString("http://127.0.0.1:", serverPort),
							Token:       "test-token",
							RealmID:     "port-test",
							STUNServers: stunServers,
						},
						ListenPorts: []uint16{8340, 8341, 8342},
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
		},
	})
	require.NotNil(t, instance, "instance should start with realm port range configured")

	conn1, err := net.ListenPacket("udp", F.ToString("127.0.0.1:8340"))
	require.Error(t, err, "first listen_ports entry should be occupied by realm runtime")
	if conn1 != nil {
		conn1.Close()
	}

	conn2, err := net.ListenPacket("udp", F.ToString("127.0.0.1:8341"))
	require.NoError(t, err, "second listen_ports entry should remain available when first succeeds")
	if err == nil {
		conn2.Close()
	}

	conn3, err := net.ListenPacket("udp", F.ToString("127.0.0.1:8342"))
	require.NoError(t, err, "third listen_ports entry should remain available when first succeeds")
	if err == nil {
		conn3.Close()
	}
}

func TestHysteria2Realm_PortRangeExhaustion(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	stunServers := startTestRealmSTUNServer(t)
	startTestRealmControlService(t, serverPort, "test-token")

	instance := startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeHysteria2,
				Options: &option.Hysteria2InboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					UpMbps:   100,
					DownMbps: 100,
					Users: []option.Hysteria2User{{
						Password: "password",
					}},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
					},
					Realm: &option.Hysteria2InboundRealm{
						Hysteria2Realm: option.Hysteria2Realm{
							ServerURL:   F.ToString("http://127.0.0.1:", serverPort),
							Token:       "test-token",
							RealmID:     "exhaust-test",
							STUNServers: stunServers,
						},
						ListenPorts: []uint16{9350},
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
		},
	})
	require.NotNil(t, instance, "instance should start with single-port realm configured")

	conn, err := net.ListenPacket("udp", F.ToString("127.0.0.1:9350"))
	require.Error(t, err, "realm runtime should occupy configured single listen port")
	if conn != nil {
		conn.Close()
	}

	instance2, err := startInstanceE(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeHysteria2,
				Options: &option.Hysteria2InboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort + 1,
					},
					UpMbps:   100,
					DownMbps: 100,
					Users: []option.Hysteria2User{{
						Password: "password",
					}},
					Realm: &option.Hysteria2InboundRealm{
						Hysteria2Realm: option.Hysteria2Realm{
							ServerURL:   F.ToString("http://127.0.0.1:", serverPort+1),
							Token:       "test-token",
							RealmID:     "exhaust-test-2",
							STUNServers: stunServers,
						},
						ListenPorts: []uint16{9350},
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
		},
	})
	if instance2 != nil {
		instance2.Close()
	}
	require.Error(t, err, "second realm inbound should fail when all listen_ports are exhausted")
}

func startInstanceE(t *testing.T, options option.Options) (*box.Box, error) {
	if options.Log == nil {
		options.Log = &option.LogOptions{}
	}
	if options.Log.Level == "" {
		options.Log.Level = "warning"
	}
	ctx, cancel := context.WithCancel(context.Background())
	instance, err := box.New(box.Options{
		Context: ctx,
		Options: options,
	})
	if err != nil {
		cancel()
		return nil, err
	}
	err = instance.Start()
	if err != nil {
		instance.Close()
		cancel()
		return nil, err
	}
	t.Cleanup(func() {
		instance.Close()
		cancel()
	})
	return instance, nil
}

func TestHysteria2Realm_authRejection(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	stunServers := startTestRealmSTUNServer(t)
	startTestRealmControlService(t, serverPort, "correct-token")

	instance := startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeHysteria2,
				Options: &option.Hysteria2InboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					UpMbps:   100,
					DownMbps: 100,
					Users: []option.Hysteria2User{{
						Password: "password",
					}},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
					},
					Realm: &option.Hysteria2InboundRealm{
						Hysteria2Realm: option.Hysteria2Realm{
							ServerURL:   F.ToString("http://127.0.0.1:", serverPort),
							Token:       "correct-token",
							RealmID:     "auth-test",
							STUNServers: stunServers,
						},
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
		},
	})
	require.NotNil(t, instance, "instance with correct token should start successfully")

	wrongTokenInstance, err := startInstanceE(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort + 1,
					},
				},
			},
			{
				Type: C.TypeHysteria2,
				Options: &option.Hysteria2InboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort + 1,
					},
					UpMbps:   100,
					DownMbps: 100,
					Users: []option.Hysteria2User{{
						Password: "password",
					}},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
					},
					Realm: &option.Hysteria2InboundRealm{
						Hysteria2Realm: option.Hysteria2Realm{
							ServerURL:   F.ToString("http://127.0.0.1:", serverPort+1),
							Token:       "wrong-token",
							RealmID:     "auth-test",
							STUNServers: stunServers,
						},
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
		},
	})

	if wrongTokenInstance != nil {
		wrongTokenInstance.Close()
	}
	require.Error(t, err, "instance with wrong token should fail to start when realm auth is implemented")
}

func TestHysteria2Realm_PreferIPv4Fallback(t *testing.T) {
	dialer := newRecordingRealmDialer()
	wrapped, err := hysteria2protocol.NewRealmPreferDialer(dialer, "prefer_ipv4", "20ms")
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = wrapped.ListenPacket(ctx, M.ParseSocksaddrHostPort("localhost", 3478))
	require.Error(t, err)
	require.Equal(t, netip.MustParseAddr("127.0.0.1"), (<-dialer.attempts).Unmap(), "prefer_ipv4 should try IPv4 before fallback")
	select {
	case attempt := <-dialer.attempts:
		require.Equal(t, netip.MustParseAddr("::1"), attempt.Unmap(), "fallback_timeout should allow IPv6 after preferred IPv4 fails")
	case <-time.After(time.Second):
		t.Fatal("fallback IPv6 attempt was not observed")
	}

	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	stunServers := startTestRealmSTUNServer(t)
	startTestRealmControlService(t, serverPort, "test-token")

	instance := startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeHysteria2,
				Options: &option.Hysteria2InboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					UpMbps:   100,
					DownMbps: 100,
					Users: []option.Hysteria2User{{
						Password: "password",
					}},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
					},
					Realm: &option.Hysteria2InboundRealm{
						Hysteria2Realm: option.Hysteria2Realm{
							ServerURL:       F.ToString("http://127.0.0.1:", serverPort),
							Token:           "test-token",
							RealmID:         "prefer-ip-test",
							PreferIPVersion: "prefer_ipv4",
							STUNServers:     stunServers,
							FallbackTimeout: "5s",
						},
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
		},
	})
	require.NotNil(t, instance, "instance should start with prefer_ipv4 fallback_timeout=5s")
}

func TestHysteria2Realm_PreferIPv6Only(t *testing.T) {
	dialer := newRecordingRealmDialer()
	wrapped, err := hysteria2protocol.NewRealmPreferDialer(dialer, "ipv6_only", "0s")
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = wrapped.ListenPacket(ctx, M.ParseSocksaddrHostPort("localhost", 3478))
	require.Error(t, err)
	require.Equal(t, netip.MustParseAddr("::1"), (<-dialer.attempts).Unmap(), "ipv6_only should only try IPv6 addresses")
	select {
	case attempt := <-dialer.attempts:
		t.Fatalf("ipv6_only unexpectedly tried non-IPv6 fallback address: %s", attempt)
	default:
	}

	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	stunServers := startTestRealmSTUNServer(t)
	startTestRealmControlService(t, serverPort, "test-token")

	instance := startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeHysteria2,
				Options: &option.Hysteria2InboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					UpMbps:   100,
					DownMbps: 100,
					Users: []option.Hysteria2User{{
						Password: "password",
					}},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
					},
					Realm: &option.Hysteria2InboundRealm{
						Hysteria2Realm: option.Hysteria2Realm{
							ServerURL:       F.ToString("http://127.0.0.1:", serverPort),
							Token:           "test-token",
							RealmID:         "ipv6-only-test",
							PreferIPVersion: "ipv6_only",
							STUNServers:     stunServers,
							FallbackTimeout: "0s",
						},
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
		},
	})
	require.NotNil(t, instance, "instance should start with ipv6_only fallback_timeout=0")
}

func TestHysteria2Realm_heartbeatEvents(t *testing.T) {
	startTestRealmControlService(t, serverPort, "test-token")

	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, F.ToString("127.0.0.1:", serverPort))
			},
		},
	}
	registerURL := F.ToString("http://127.0.0.1:", serverPort, "/v1/events-test/")
	registerReq, err := http.NewRequest(http.MethodPost, registerURL, bytes.NewBufferString(`{"addresses":["127.0.0.1:12345"]}`))
	require.NoError(t, err)
	registerReq.Header.Set("Authorization", "Bearer test-token")
	registerReq.Header.Set("Content-Type", "application/json")
	registerResp, err := httpClient.Do(registerReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, registerResp.StatusCode)
	var registerPayload struct {
		SessionID string `json:"session_id"`
	}
	require.NoError(t, json.NewDecoder(registerResp.Body).Decode(&registerPayload))
	registerResp.Body.Close()
	require.NotEmpty(t, registerPayload.SessionID)

	eventsURL := F.ToString("http://127.0.0.1:", serverPort, "/v1/events-test/events")
	req, err := http.NewRequest(http.MethodGet, eventsURL, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+registerPayload.SessionID)
	resp, err := httpClient.Do(req)
	require.NoError(t, err, "realm events endpoint should be accessible at /v1/{realm_id}/events")
	require.NotNil(t, resp, "realm events endpoint should return a response")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestHysteria2Realm_outboundConnect(t *testing.T) {
	_, certPem, _ := createSelfSignedCertificate(t, "example.org")
	stunServers := startTestRealmSTUNServer(t)
	startTestRealmControlService(t, serverPort, "client-token")

	instance := startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
			{
				Type: C.TypeHysteria2,
				Tag:  "hy2-out-realm",
				Options: &option.Hysteria2OutboundOptions{
					Password: "password",
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
						TLS: &option.OutboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
						},
					},
					Realm: &option.Hysteria2Realm{
						ServerURL:   F.ToString("http://127.0.0.1:", serverPort),
						Token:       "client-token",
						RealmID:     "remote-realm",
						STUNServers: stunServers,
					},
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							Inbound: []string{"mixed-in"},
						},
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeRoute,
							RouteOptions: option.RouteActionOptions{
								Outbound: "hy2-out-realm",
							},
						},
					},
				},
			},
		},
	})
	require.NotNil(t, instance, "instance should start with realm outbound configured")
}

func TestHysteria2Realm_staticDirectHy2Regression(t *testing.T) {
	testHysteria2Self(t, "", false)
}
