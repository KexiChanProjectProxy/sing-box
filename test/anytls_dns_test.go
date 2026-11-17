package main

import (
	"context"
	"net"
	"net/netip"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"

	mDNS "github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

const anyTLSDNSTestDomain = "dns.test.example."

// testAuthority is a local DNS server answering anyTLSDNSTestDomain with
// 127.0.0.1 on two UDP ports, counting the A queries it receives. It uses UDP
// only because the TCP DNS client sends an extra query of its own the first
// time, to probe for connection reuse, which would spoil the count.
type testAuthority struct {
	queries atomic.Int32
	addrs   [2]string
}

func startTestAuthority(t *testing.T) *testAuthority {
	t.Helper()
	authority := &testAuthority{}
	handler := mDNS.HandlerFunc(func(writer mDNS.ResponseWriter, request *mDNS.Msg) {
		response := new(mDNS.Msg)
		response.SetReply(request)
		question := request.Question[0]
		if question.Name == anyTLSDNSTestDomain && question.Qtype == mDNS.TypeA {
			authority.queries.Add(1)
			response.Answer = append(response.Answer, &mDNS.A{
				Hdr: mDNS.RR_Header{Name: question.Name, Rrtype: mDNS.TypeA, Class: mDNS.ClassINET, Ttl: 600},
				A:   net.IPv4(127, 0, 0, 1),
			})
		}
		_ = writer.WriteMsg(response)
	})
	for index := range authority.addrs {
		packetConn, err := net.ListenPacket("udp", "127.0.0.1:0")
		require.NoError(t, err)
		authority.addrs[index] = packetConn.LocalAddr().String()
		server := &mDNS.Server{PacketConn: packetConn, Handler: handler}
		go server.ActivateAndServe()
		t.Cleanup(func() {
			_ = server.Shutdown()
		})
	}
	return authority
}

func authorityServer(t *testing.T, serverType string, tag string, address string) option.DNSServerOptions {
	addrPort, err := netip.ParseAddrPort(address)
	require.NoError(t, err)
	return option.DNSServerOptions{
		Type: serverType,
		Tag:  tag,
		Options: &option.RemoteDNSServerOptions{
			DNSServerAddressOptions: option.DNSServerAddressOptions{
				Server:     addrPort.Addr().String(),
				ServerPort: addrPort.Port(),
			},
		},
	}
}

// anyTLSDNSServer starts a server whose anytls inbound answers DNS, when dns is
// not nil, with a group of the authority's two servers behind it. Its
// mixed inbound on otherClientPort lets the test make the server resolve a
// name itself.
func anyTLSDNSServer(t *testing.T, authority *testAuthority, dns *option.AnyTLSInboundDNSOptions, certPem, keyPem string) {
	t.Helper()
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeAnyTLS,
				Tag:  "anytls-in",
				Options: &option.AnyTLSInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Users: []option.AnyTLSUser{{Name: "sekai", Password: "password"}},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
					},
					DNS: dns,
				},
			},
			{
				Type: C.TypeMixed,
				Tag:  "server-mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: otherClientPort,
					},
				},
			},
		},
		Outbounds: []option.Outbound{{Type: C.TypeDirect}},
		DNS: &option.DNSOptions{RawDNSOptions: option.RawDNSOptions{
			Servers: []option.DNSServerOptions{
				authorityServer(t, C.DNSTypeUDP, "authority-1", authority.addrs[0]),
				authorityServer(t, C.DNSTypeUDP, "authority-2", authority.addrs[1]),
				{
					Type: C.DNSTypeGroup,
					Tag:  "up",
					Options: &option.GroupDNSServerOptions{
						Servers: []string{"authority-1", "authority-2"},
					},
				},
			},
			Final: "up",
		}},
		Route: &option.RouteOptions{
			DefaultDomainResolver: &option.DomainResolveOptions{
				Server:   "up",
				Strategy: option.DomainStrategy(C.DomainStrategyIPv4Only),
			},
		},
	})
}

// anyTLSDNSClient starts a client that resolves through its anytls outbound
// and answers plain DNS on otherPort with it.
func anyTLSDNSClient(t *testing.T, certPem string) {
	t.Helper()
	startInstance(t, option.Options{
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
				Type: C.TypeDirect,
				Tag:  "dns-in",
				Options: &option.DirectInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: otherPort,
					},
					Network: option.NetworkList(N.NetworkUDP),
				},
			},
		},
		Outbounds: []option.Outbound{
			{Type: C.TypeDirect},
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
							CertificatePath: certPem,
						},
					},
				},
			},
		},
		DNS: &option.DNSOptions{RawDNSOptions: option.RawDNSOptions{
			Servers: []option.DNSServerOptions{{
				Type:    C.DNSTypeAnyTLS,
				Tag:     "remote",
				Options: &option.AnyTLSDNSServerOptions{Outbound: "anytls-out"},
			}},
			Final: "remote",
		}},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{Inbound: []string{"dns-in"}},
						RuleAction:     option.RuleAction{Action: C.RuleActionTypeHijackDNS},
					},
				},
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{Inbound: []string{"mixed-in"}},
						RuleAction: option.RuleAction{
							Action:       C.RuleActionTypeRoute,
							RouteOptions: option.RouteActionOptions{Outbound: "anytls-out"},
						},
					},
				},
			},
		},
	})
}

func queryClientDNS(t *testing.T) (*mDNS.Msg, error) {
	t.Helper()
	client := &mDNS.Client{Net: "udp", Timeout: 3 * time.Second}
	request := new(mDNS.Msg)
	request.SetQuestion(anyTLSDNSTestDomain, mDNS.TypeA)
	response, _, err := client.Exchange(request, net.JoinHostPort("127.0.0.1", strconv.Itoa(int(otherPort))))
	return response, err
}

func requireTestAnswer(t *testing.T, response *mDNS.Msg) {
	t.Helper()
	require.Equal(t, mDNS.RcodeSuccess, response.Rcode)
	require.Len(t, response.Answer, 1)
	record, isA := response.Answer[0].(*mDNS.A)
	require.True(t, isA, "answer %v", response.Answer[0])
	require.Equal(t, "127.0.0.1", record.A.String())
}

func TestAnyTLSDNS(t *testing.T) {
	for name, server := range map[string]string{
		"server":    "up",
		"dns_rules": "",
	} {
		t.Run(name, func(t *testing.T) {
			_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
			authority := startTestAuthority(t)
			anyTLSDNSServer(t, authority, &option.AnyTLSInboundDNSOptions{Enabled: true, Server: server}, certPem, keyPem)
			anyTLSDNSClient(t, certPem)

			response, err := queryClientDNS(t)
			require.NoError(t, err)
			requireTestAnswer(t, response)
			require.EqualValues(t, 1, authority.queries.Load())

			// The server resolving the name for itself answers from the cache
			// the in-session query filled.
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			defer listener.Close()
			go func() {
				conn, acceptErr := listener.Accept()
				if acceptErr == nil {
					conn.Close()
				}
			}()
			port := uint16(listener.Addr().(*net.TCPAddr).Port)
			dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", otherClientPort), socks.Version5, "", "")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			conn, err := dialer.DialContext(ctx, N.NetworkTCP, M.ParseSocksaddrHostPort("dns.test.example", port))
			require.NoError(t, err)
			conn.Close()
			require.EqualValues(t, 1, authority.queries.Load(), "the server's own lookup did not share the cache")

			// Proxied traffic is unaffected.
			testTCP(t, clientPort, testPort)
		})
	}
}

// Against a server that does not offer DNS, queries fail while proxied
// traffic keeps working.
func TestAnyTLSDNSServerWithoutDNS(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	authority := startTestAuthority(t)
	anyTLSDNSServer(t, authority, nil, certPem, keyPem)
	anyTLSDNSClient(t, certPem)

	response, err := queryClientDNS(t)
	if err == nil {
		require.NotEqual(t, mDNS.RcodeSuccess, response.Rcode)
	}
	require.EqualValues(t, 0, authority.queries.Load())
	testTCP(t, clientPort, testPort)
}
