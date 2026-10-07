//go:build with_quic && with_gvisor

package cloudflarewarp

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/warpapi"
	"github.com/sagernet/sing-box/experimental/cachefile"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/cloudflarewarp/warptest"
	"github.com/sagernet/sing-box/transport/ipstack"
	tun "github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"

	"github.com/stretchr/testify/require"
)

var (
	clientInet4 = netip.MustParsePrefix("172.16.0.2/32")
	clientInet6 = netip.MustParsePrefix("fd00::2/128")
	echoInet4   = netip.MustParseAddr("10.0.0.1")
)

// echoHandler echoes every TCP stream and UDP datagram it is given.
type echoHandler struct{}

func (echoHandler) JudgeFlow(uint8, netip.AddrPort, netip.AddrPort, []byte) tun.FlowVerdict {
	return tun.FlowVerdict{Action: tun.ActionAccept}
}

func (echoHandler) NewDNSPacket([]byte, M.Socksaddr, M.Socksaddr, N.PacketWriter) {}

func (echoHandler) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	go func() {
		io.Copy(conn, conn)
		conn.Close()
	}()
}

func (echoHandler) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	go func() {
		defer conn.Close()
		for {
			buffer := buf.NewPacket()
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			from, err := conn.ReadPacket(buffer)
			if err != nil {
				buffer.Release()
				return
			}
			err = conn.WritePacket(buffer, from)
			if err != nil {
				return
			}
		}
	}()
}

// serveEcho bridges server sessions into a gVisor stack that echoes traffic.
func serveEcho(t *testing.T, server *warptest.Server) {
	t.Helper()
	device, err := ipstack.NewDevice(ipstack.DeviceOptions{
		Context:    context.Background(),
		Logger:     log.NewNOPFactory().Logger(),
		Handler:    echoHandler{},
		UDPTimeout: time.Minute,
		Addresses:  []netip.Prefix{netip.PrefixFrom(echoInet4, 32)},
	})
	require.NoError(t, err)
	require.NoError(t, device.Start())
	sessions := make(chan *warptest.Session, 4)
	var current *warptest.Session
	device.SetPacketWriter(func(packetBuffers []*buf.Buffer) error {
		defer buf.ReleaseMulti(packetBuffers)
		select {
		case session := <-sessions:
			current = session
		default:
		}
		if current == nil {
			return nil
		}
		for _, packetBuffer := range packetBuffers {
			current.WritePacket(packetBuffer.Bytes())
		}
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		device.Close()
	})
	go func() {
		for {
			var session *warptest.Session
			select {
			case session = <-server.Sessions:
			case <-ctx.Done():
				return
			}
			sessions <- session
			go func() {
				for {
					packet, err := session.ReadPacket(ctx)
					if err != nil {
						return
					}
					device.WriteInboundBuffers([]*buf.Buffer{buf.As(packet)})
				}
			}()
		}
	}()
}

func startOutbound(t *testing.T, ctx context.Context, options option.CloudflareWARPOutboundOptions) *Outbound {
	t.Helper()
	require.NoError(t, options.Validate())
	created, err := NewOutbound(ctx, nil, log.NewNOPFactory().Logger(), "warp", options)
	require.NoError(t, err)
	outbound := created.(*Outbound)
	for _, stage := range adapter.ListStartStages {
		require.NoError(t, outbound.Start(stage))
	}
	t.Cleanup(func() { outbound.Close() })
	return outbound
}

func assertEcho(t *testing.T, outbound *Outbound) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := outbound.DialContext(ctx, N.NetworkTCP, M.SocksaddrFrom(echoInet4, 80))
	require.NoError(t, err)
	message := []byte("hello through warp")
	_, err = conn.Write(message)
	require.NoError(t, err)
	reply := make([]byte, len(message))
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err = io.ReadFull(conn, reply)
	require.NoError(t, err)
	require.Equal(t, message, reply)
	conn.Close()

	destination := M.SocksaddrFrom(echoInet4, 53)
	packetConn, err := outbound.ListenPacket(ctx, destination)
	require.NoError(t, err)
	defer packetConn.Close()
	_, err = packetConn.WriteTo([]byte("udp ping"), destination.UDPAddr())
	require.NoError(t, err)
	packetConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	udpReply := make([]byte, 64)
	n, _, err := packetConn.ReadFrom(udpReply)
	require.NoError(t, err)
	require.Equal(t, "udp ping", string(udpReply[:n]))
}

func encodePublicKey(t *testing.T, key *ecdsa.PublicKey) string {
	der, err := x509.MarshalPKIXPublicKey(key)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(der)
}

func TestOutboundStaticCredentials(t *testing.T) {
	clientKey, err := warpapi.GeneratePrivateKey()
	require.NoError(t, err)
	encodedKey, err := warpapi.EncodePrivateKey(clientKey)
	require.NoError(t, err)
	server, err := warptest.NewServer(warptest.Options{ClientPublicKey: &clientKey.PublicKey})
	require.NoError(t, err)
	defer server.Close()
	serveEcho(t, server)

	outbound := startOutbound(t, context.Background(), option.CloudflareWARPOutboundOptions{
		ServerOptions:     option.ServerOptions{Server: "127.0.0.1", ServerPort: uint16(server.Addr().Port)},
		PrivateKey:        encodedKey,
		Address:           badoption.Listable[netip.Prefix]{clientInet4, clientInet6},
		EndpointPublicKey: encodePublicKey(t, server.PublicKey),
	})
	assertEcho(t, outbound)

	// A server-side close must be followed by an automatic reconnect.
	before := len(server.Requests())
	outbound.InterfaceUpdated(context.Background())
	require.Eventually(t, func() bool { return len(server.Requests()) > before }, 10*time.Second, 50*time.Millisecond)
	assertEcho(t, outbound)
}

func TestOutboundAutoRegistration(t *testing.T) {
	server, err := warptest.NewServer(warptest.Options{})
	require.NoError(t, err)
	defer server.Close()
	serveEcho(t, server)

	var registrations int
	api := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/reg":
			registrations++
			writer.Write([]byte(`{"id":"device-1","token":"token-1","account":{"id":"account-1"}}`))
		case request.Method == http.MethodPatch && request.URL.Path == "/reg/device-1":
			json.NewEncoder(writer).Encode(map[string]any{
				"id": "device-1",
				"config": map[string]any{
					"peers": []any{map[string]any{
						"public_key": encodePublicKey(t, server.PublicKey),
						"endpoint":   map[string]any{"v4": "127.0.0.1:0"},
					}},
					"interface": map[string]any{"addresses": map[string]any{"v4": clientInet4.Addr().String()}},
				},
			})
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer api.Close()
	previousBaseURL := apiBaseURL
	apiBaseURL = api.URL
	testAPIHTTPClient = api.Client()
	t.Cleanup(func() {
		apiBaseURL = previousBaseURL
		testAPIHTTPClient = nil
	})

	cachePath := filepath.Join(t.TempDir(), "cache.db")
	options := option.CloudflareWARPOutboundOptions{
		ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: uint16(server.Addr().Port)},
	}
	// Mirror box.New: outbounds are created before the cache file service is
	// registered, and the cache file opens at the initialize stage.
	start := func() (*Outbound, *cachefile.CacheFile) {
		ctx := service.ContextWithDefaultRegistry(context.Background())
		created, err := NewOutbound(ctx, nil, log.NewNOPFactory().Logger(), "warp", options)
		require.NoError(t, err)
		outbound := created.(*Outbound)
		t.Cleanup(func() { outbound.Close() })
		cacheFile := cachefile.New(ctx, log.NewNOPFactory().Logger(), option.CacheFileOptions{Path: cachePath})
		service.MustRegister[adapter.CacheFile](ctx, cacheFile)
		require.NoError(t, cacheFile.Start(adapter.StartStateInitialize))
		for _, stage := range adapter.ListStartStages {
			require.NoError(t, outbound.Start(stage))
		}
		return outbound, cacheFile
	}

	outbound, cacheFile := start()
	assertEcho(t, outbound)
	require.Equal(t, 1, registrations)
	data, err := cacheFile.LoadCloudflareWARPRegistration("warp")
	require.NoError(t, err)
	var registration warpapi.Registration
	require.NoError(t, json.Unmarshal(data, &registration))
	require.Equal(t, "device-1", registration.DeviceID)
	require.NoError(t, outbound.Close())
	require.NoError(t, cacheFile.Close())

	// A restart reuses the cached device instead of registering again.
	outbound, cacheFile = start()
	defer cacheFile.Close()
	assertEcho(t, outbound)
	require.Equal(t, 1, registrations)
}

func TestOutboundRequiresCacheFileForAutoRegistration(t *testing.T) {
	created, err := NewOutbound(context.Background(), nil, log.NewNOPFactory().Logger(), "warp", option.CloudflareWARPOutboundOptions{})
	require.NoError(t, err)
	defer created.(*Outbound).Close()
	err = created.(*Outbound).Start(adapter.StartStateStart)
	require.ErrorContains(t, err, "experimental.cache_file.enabled")
}

func TestOutboundNotReady(t *testing.T) {
	clientKey, err := warpapi.GeneratePrivateKey()
	require.NoError(t, err)
	encodedKey, err := warpapi.EncodePrivateKey(clientKey)
	require.NoError(t, err)
	created, err := NewOutbound(context.Background(), nil, log.NewNOPFactory().Logger(), "warp", option.CloudflareWARPOutboundOptions{
		ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: 1},
		PrivateKey:    encodedKey,
		Address:       badoption.Listable[netip.Prefix]{clientInet4},
	})
	require.NoError(t, err)
	defer created.(*Outbound).Close()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err = created.DialContext(ctx, N.NetworkTCP, M.SocksaddrFrom(echoInet4, 80))
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestOutboundEphemeralRegistration(t *testing.T) {
	server, err := warptest.NewServer(warptest.Options{})
	require.NoError(t, err)
	defer server.Close()
	serveEcho(t, server)

	var (
		access        sync.Mutex
		registrations int
		deleted       []string
	)
	api := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		access.Lock()
		defer access.Unlock()
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/reg":
			registrations++
			fmt.Fprintf(writer, `{"id":"device-%d","token":"token-%d","account":{"id":"account"}}`, registrations, registrations)
		case request.Method == http.MethodPatch && strings.HasPrefix(request.URL.Path, "/reg/"):
			json.NewEncoder(writer).Encode(map[string]any{
				"id": strings.TrimPrefix(request.URL.Path, "/reg/"),
				"config": map[string]any{
					"peers": []any{map[string]any{
						"public_key": encodePublicKey(t, server.PublicKey),
						"endpoint":   map[string]any{"v4": "127.0.0.1:0"},
					}},
					"interface": map[string]any{"addresses": map[string]any{"v4": clientInet4.Addr().String()}},
				},
			})
		case request.Method == http.MethodDelete && strings.HasPrefix(request.URL.Path, "/reg/"):
			require.Equal(t, "Bearer token-"+strings.TrimPrefix(request.URL.Path, "/reg/device-"), request.Header.Get("Authorization"))
			deleted = append(deleted, strings.TrimPrefix(request.URL.Path, "/reg/"))
			writer.WriteHeader(http.StatusNoContent)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer api.Close()
	previousBaseURL := apiBaseURL
	apiBaseURL = api.URL
	testAPIHTTPClient = api.Client()
	t.Cleanup(func() {
		apiBaseURL = previousBaseURL
		testAPIHTTPClient = nil
	})

	options := option.CloudflareWARPOutboundOptions{
		ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: uint16(server.Addr().Port)},
		Ephemeral:     true,
	}
	for run := 1; run <= 2; run++ {
		// No cache file is registered in the context.
		outbound := startOutbound(t, service.ContextWithDefaultRegistry(context.Background()), options)
		assertEcho(t, outbound)
		require.NoError(t, outbound.Close())
		access.Lock()
		require.Equal(t, run, registrations)
		require.Equal(t, fmt.Sprintf("device-%d", run), deleted[len(deleted)-1])
		require.Len(t, deleted, run)
		access.Unlock()
	}
}
