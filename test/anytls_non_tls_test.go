package main

import (
	"context"
	stdtls "crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/netip"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"

	"github.com/stretchr/testify/require"
)

type sharedPortShadowsocks struct {
	method         string
	serverPassword string
	users          []option.ShadowsocksUser
	clientPassword string
}

func newSharedPortShadowsocks(t *testing.T, method string, keyLength int, multiUser bool) sharedPortShadowsocks {
	password := mkBase64(t, keyLength)
	if !multiUser {
		return sharedPortShadowsocks{method: method, serverPassword: password, clientPassword: password}
	}
	userPassword := mkBase64(t, keyLength)
	return sharedPortShadowsocks{
		method:         method,
		serverPassword: password,
		users:          []option.ShadowsocksUser{{Name: "sekai", Password: userPassword}},
		clientPassword: password + ":" + userPassword,
	}
}

// sharedPortOptions puts an anytls inbound and a shadowsocks inbound on
// serverPort, the anytls one handing non-TLS connections to the other.
//
// Clients reach the anytls outbound through a mixed inbound on clientPort and
// the shadowsocks outbound through one on otherClientPort. The server routes
// testPort only for anytls-in and otherPort only for ss-in, and rejects
// everything else, so traffic attributed to the wrong inbound fails.
func sharedPortOptions(t *testing.T, ss sharedPortShadowsocks, caPem string, certPem string, keyPem string) option.Options {
	t.Helper()
	listen := common.Ptr(badoption.Addr(netip.IPv4Unspecified()))
	routeRule := func(inbound string, port uint16, outbound string) option.Rule {
		rule := option.Rule{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultRule{
				RawDefaultRule: option.RawDefaultRule{
					Inbound: []string{inbound},
				},
				RuleAction: option.RuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.RouteActionOptions{Outbound: outbound},
				},
			},
		}
		if port != 0 {
			rule.DefaultOptions.Port = []uint16{port}
		}
		return rule
	}
	return option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-anytls",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{Listen: listen, ListenPort: clientPort},
				},
			},
			{
				Type: C.TypeMixed,
				Tag:  "mixed-ss",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{Listen: listen, ListenPort: otherClientPort},
				},
			},
			{
				Type: C.TypeAnyTLS,
				Tag:  "anytls-in",
				Options: &option.AnyTLSInboundOptions{
					ListenOptions: option.ListenOptions{Listen: listen, ListenPort: serverPort},
					Users:         []option.AnyTLSUser{{Name: "sekai", Password: "password"}},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
					},
					NonTLSHandler: "ss-in",
				},
			},
			{
				Type: C.TypeShadowsocks,
				Tag:  "ss-in",
				Options: &option.ShadowsocksInboundOptions{
					ListenOptions: option.ListenOptions{Listen: listen, ListenPort: serverPort},
					Network:       N.NetworkUDP,
					Method:        ss.method,
					Password:      ss.serverPassword,
					Users:         ss.users,
				},
			},
		},
		Outbounds: []option.Outbound{
			{Type: C.TypeDirect, Tag: "direct"},
			{
				Type: C.TypeAnyTLS,
				Tag:  "anytls-out",
				Options: &option.AnyTLSOutboundOptions{
					ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: serverPort},
					Password:      "password",
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
						TLS: &option.OutboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: caPem,
						},
					},
				},
			},
			{
				Type: C.TypeShadowsocks,
				Tag:  "ss-out",
				Options: &option.ShadowsocksOutboundOptions{
					ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: serverPort},
					Method:        ss.method,
					Password:      ss.clientPassword,
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				routeRule("mixed-anytls", 0, "anytls-out"),
				routeRule("mixed-ss", 0, "ss-out"),
				routeRule("anytls-in", testPort, "direct"),
				routeRule("ss-in", otherPort, "direct"),
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							Network: []string{N.NetworkTCP, N.NetworkUDP},
						},
						RuleAction: option.RuleAction{
							Action:        C.RuleActionTypeReject,
							RejectOptions: option.RejectActionOptions{Method: C.RuleActionRejectMethodDefault},
						},
					},
				},
			},
		},
	}
}

func TestAnyTLSNonTLSHandler(t *testing.T) {
	for _, tc := range []struct {
		name      string
		method    string
		keyLength int
		multiUser bool
	}{
		{"2022-blake3-aes-128-gcm", "2022-blake3-aes-128-gcm", 16, false},
		{"2022-blake3-aes-128-gcm-multi-user", "2022-blake3-aes-128-gcm", 16, true},
		{"aes-128-gcm", "aes-128-gcm", 16, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			caPem, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
			ss := newSharedPortShadowsocks(t, tc.method, tc.keyLength, tc.multiUser)
			startInstance(t, sharedPortOptions(t, ss, caPem, certPem, keyPem))
			testTCP(t, clientPort, testPort)
			testSuit(t, otherClientPort, otherPort)
			// The other inbound's port is rejected, so each protocol was
			// routed as coming from its own inbound.
			requireRejected(t, clientPort, otherPort)
			requireRejected(t, otherClientPort, testPort)
		})
	}
}

func requireRejected(t *testing.T, socksPort uint16, destinationPort uint16) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:"+F.ToString(destinationPort))
	require.NoError(t, err)
	defer listener.Close()
	accepted := make(chan struct{}, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			conn.Close()
			accepted <- struct{}{}
		}
	}()
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", socksPort), socks.Version5, "", "")
	conn, err := dialer.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("127.0.0.1", destinationPort))
	if err == nil {
		_, _ = conn.Write([]byte("ping"))
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _ = conn.Read(make([]byte, 1))
		conn.Close()
	}
	select {
	case <-accepted:
		t.Fatalf("connection through socks port %d reached port %d", socksPort, destinationPort)
	case <-time.After(500 * time.Millisecond):
	}
}

// TestAnyTLSNonTLSHandlerConcurrent opens 1000 connections to the shared
// port at once, half of them TLS handshakes and half Shadowsocks streams,
// and checks that every one reaches its protocol.
func TestAnyTLSNonTLSHandlerConcurrent(t *testing.T) {
	caPem, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	ss := newSharedPortShadowsocks(t, "2022-blake3-aes-128-gcm", 16, false)
	startInstance(t, sharedPortOptions(t, ss, caPem, certPem, keyPem))

	echo, err := net.Listen("tcp", "127.0.0.1:"+F.ToString(otherPort))
	require.NoError(t, err)
	defer echo.Close()
	go func() {
		for {
			conn, acceptErr := echo.Accept()
			if acceptErr != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()

	caContent, err := os.ReadFile(caPem)
	require.NoError(t, err)
	rootCAs := x509.NewCertPool()
	require.True(t, rootCAs.AppendCertsFromPEM(caContent))
	tlsConfig := &stdtls.Config{ServerName: "example.org", RootCAs: rootCAs}
	ssDialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", otherClientPort), socks.Version5, "", "")

	const connections = 1000
	var (
		wait   sync.WaitGroup
		errMu  sync.Mutex
		errs   []error
		start  = make(chan struct{})
		record = func(err error) {
			if err != nil {
				errMu.Lock()
				errs = append(errs, err)
				errMu.Unlock()
			}
		}
	)
	for i := 0; i < connections; i++ {
		wait.Add(1)
		if i%2 == 0 {
			go func() {
				defer wait.Done()
				<-start
				dialer := &net.Dialer{Timeout: 10 * time.Second}
				conn, err := stdtls.DialWithDialer(dialer, "tcp", "127.0.0.1:"+F.ToString(serverPort), tlsConfig)
				if err != nil {
					record(wrapErr("tls", err))
					return
				}
				conn.Close()
			}()
		} else {
			go func(i int) {
				defer wait.Done()
				<-start
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				conn, err := ssDialer.DialContext(ctx, N.NetworkTCP, M.ParseSocksaddrHostPort("127.0.0.1", otherPort))
				if err != nil {
					record(wrapErr("shadowsocks dial", err))
					return
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
				payload := []byte("shadowsocks " + F.ToString(i))
				_, err = conn.Write(payload)
				if err != nil {
					record(wrapErr("shadowsocks write", err))
					return
				}
				reply := make([]byte, len(payload))
				_, err = io.ReadFull(conn, reply)
				if err != nil {
					record(wrapErr("shadowsocks read", err))
					return
				}
				if string(reply) != string(payload) {
					record(errors.New("shadowsocks: echo mismatch"))
				}
			}(i)
		}
	}
	close(start)
	wait.Wait()
	require.Empty(t, errs)
}

func wrapErr(prefix string, err error) error {
	return errors.New(prefix + ": " + err.Error())
}

func TestAnyTLSNonTLSHandlerOptions(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	tlsOptions := &option.InboundTLSOptions{
		Enabled:         true,
		ServerName:      "example.org",
		CertificatePath: certPem,
		KeyPath:         keyPem,
	}
	anyTLS := func(tls *option.InboundTLSOptions, handler string) option.Inbound {
		return option.Inbound{
			Type: C.TypeAnyTLS,
			Tag:  "anytls-in",
			Options: &option.AnyTLSInboundOptions{
				ListenOptions: option.ListenOptions{
					Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
					ListenPort: serverPort,
				},
				Users:                      []option.AnyTLSUser{{Name: "sekai", Password: "password"}},
				InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{TLS: tls},
				NonTLSHandler:              handler,
			},
		}
	}
	mixed := option.Inbound{
		Type: C.TypeMixed,
		Tag:  "mixed-in",
		Options: &option.HTTPMixedInboundOptions{
			ListenOptions: option.ListenOptions{
				Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
				ListenPort: clientPort,
			},
		},
	}
	for _, tc := range []struct {
		name     string
		inbounds []option.Inbound
		want     string
	}{
		{"tls disabled", []option.Inbound{anyTLS(nil, "mixed-in"), mixed}, "TLS is not enabled"},
		{"itself", []option.Inbound{anyTLS(tlsOptions, "anytls-in")}, "must not be the inbound itself"},
		{"missing", []option.Inbound{anyTLS(tlsOptions, "ss-in")}, "inbound not found: ss-in"},
		{"not shadowsocks", []option.Inbound{anyTLS(tlsOptions, "mixed-in"), mixed}, "is mixed, not shadowsocks"},
		{"method none", []option.Inbound{anyTLS(tlsOptions, "ss-in"), {
			Type: C.TypeShadowsocks,
			Tag:  "ss-in",
			Options: &option.ShadowsocksInboundOptions{
				ListenOptions: option.ListenOptions{
					Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
					ListenPort: serverPort,
				},
				Network: N.NetworkUDP,
				Method:  "none",
			},
		}}, "uses the method none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := startBoxForError(t, option.Options{Inbounds: tc.inbounds})
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

// startBoxForError creates and starts a box, returning the error from
// whichever step fails.
func startBoxForError(t *testing.T, options option.Options) error {
	t.Helper()
	options.Log = &option.LogOptions{Level: "warning"}
	ctx, cancel := context.WithCancel(globalCtx)
	t.Cleanup(cancel)
	instance, err := box.New(box.Options{Context: ctx, Options: options})
	if err != nil {
		return err
	}
	t.Cleanup(func() {
		_ = instance.Close()
	})
	return instance.Start()
}
