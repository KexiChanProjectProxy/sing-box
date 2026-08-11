package http

import (
	"bufio"
	"context"
	"net"
	std_http "net/http"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	sHTTP "github.com/sagernet/sing/protocol/http"
)

func TestDynamicHTTPPassword(t *testing.T) {
	password, err := dynamicHTTPPassword(&adapter.InboundContext{
		User:   "alice",
		Source: M.ParseSocksaddr("192.0.2.8:48123"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if password != "ccd804b11dd8a435" {
		t.Fatalf("unexpected password: %s", password)
	}
}

func TestDynamicOutboundSendsDerivedCredential(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	type credential struct {
		username string
		password string
		ok       bool
	}
	credentials := make(chan credential, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		request, err := std_http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			return
		}
		username, password, ok := sHTTP.ParseBasicAuth(request.Header.Get("Proxy-Authorization"))
		credentials <- credential{username, password, ok}
		_, _ = conn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
	}()

	server := M.ParseSocksaddr(listener.Addr().String())
	rawOutbound, err := NewDynamicOutbound(context.Background(), nil, log.NewNOPFactory().Logger(), "dynamic-http", option.HTTPDynamicOutboundOptions{
		ServerOptions: option.ServerOptions{
			Server:     server.AddrString(),
			ServerPort: server.Port,
		},
		Username: "fixed-user",
	})
	if err != nil {
		t.Fatal(err)
	}
	outbound := rawOutbound.(*DynamicOutbound)
	ctx := adapter.WithContext(context.Background(), &adapter.InboundContext{
		User:   "alice",
		Source: M.ParseSocksaddr("192.0.2.8:48123"),
	})
	conn, err := outbound.DialContext(ctx, N.NetworkTCP, M.ParseSocksaddrHostPort("example.com", 443))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	authCredential := <-credentials
	if !authCredential.ok {
		t.Fatal("missing HTTP Basic authorization")
	}
	if authCredential.username != "fixed-user" {
		t.Fatalf("unexpected username: %s", authCredential.username)
	}
	if authCredential.password != "ccd804b11dd8a435" {
		t.Fatalf("unexpected password: %s", authCredential.password)
	}
}

func TestDynamicOutboundRejectsMissingInboundCredentials(t *testing.T) {
	outbound := &DynamicOutbound{logger: log.NewNOPFactory().Logger()}
	_, err := outbound.DialContext(context.Background(), N.NetworkTCP, M.ParseSocksaddrHostPort("example.com", 443))
	if err == nil || err.Error() != "http-dynamic outbound: inbound username is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}
