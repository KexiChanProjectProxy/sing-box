package firefoxvpn

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	fakeFxAHost      = "api.accounts.firefox.com"
	fakeGuardianHost = "vpn.mozilla.org"
	fakeProxyHost    = "proxy.firefoxvpn.test"
)

type fakeFxAServer struct {
	BaseURL   string
	Hostname  string
	Port      uint16
	RootCAPEM []byte

	mu            sync.Mutex
	loginCalls    int
	sessionTokens []string
	server        *httptest.Server
}

type fakeGuardianServer struct {
	BaseURL   string
	Hostname  string
	Port      uint16
	RootCAPEM []byte

	server *httptest.Server
}

type fakeH2ProxyServer struct {
	URL            string
	ServerName     string
	Port           uint16
	CertificatePEM []byte

	ExpectedProxyPassToken string

	mu                  sync.Mutex
	connectDestinations []string
	server              *httptest.Server
}

type echoBackend struct {
	Address string
	Port    uint16

	listener net.Listener
	wg       sync.WaitGroup
}

type observableProxy struct {
	tag      string
	listener net.Listener

	mu                 sync.Mutex
	destinations       []string
	hostAliases        map[string]string
	forcedConnectError error

	wg sync.WaitGroup
}

func newFakeFxAServer(t *testing.T) *fakeFxAServer {
	t.Helper()
	serverState := &fakeFxAServer{Hostname: fakeFxAHost}
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/v1/account/login":
			require.Equal(t, http.MethodPost, request.Method)
			serverState.mu.Lock()
			serverState.loginCalls++
			sessionToken := fmt.Sprintf("%064x", serverState.loginCalls)
			serverState.sessionTokens = append(serverState.sessionTokens, sessionToken)
			serverState.mu.Unlock()
			_, _ = writer.Write([]byte(`{"sessionToken":"` + sessionToken + `","uid":"fxa-user","verified":true,"authAt":1700000000}`))
		case "/v1/oauth/token":
			require.Equal(t, http.MethodPost, request.Method)
			var tokenRequest struct {
				GrantType string `json:"grant_type"`
			}
			require.NoError(t, json.NewDecoder(request.Body).Decode(&tokenRequest))
			if tokenRequest.GrantType == "refresh_token" {
				_, _ = writer.Write([]byte(`{"access_token":"fxa-access-token-refresh","refresh_token":"fxa-refresh-token","expires_in":3600,"scope":"profile https://identity.mozilla.com/apps/vpn","token_type":"bearer"}`))
				return
			}
			_, _ = writer.Write([]byte(`{"access_token":"fxa-access-token","refresh_token":"fxa-refresh-token","expires_in":3600,"scope":"profile https://identity.mozilla.com/apps/vpn","token_type":"bearer"}`))
		default:
			http.NotFound(writer, request)
		}
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	serverState.BaseURL = server.URL
	serverState.Port = uint16(server.Listener.Addr().(*net.TCPAddr).Port)
	serverState.server = server
	return serverState
}

func newFakeGuardianServer(t *testing.T, proxyPassToken string) *fakeGuardianServer {
	t.Helper()
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer fxa-access-token", request.Header.Get("Authorization"))
		switch request.URL.Path {
		case "/api/v1/fpn/token":
			writer.Header().Set("X-Quota-Limit", "1")
			writer.Header().Set("X-Quota-Remaining", "1")
			writer.Header().Set("X-Quota-Reset", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
			_, _ = writer.Write([]byte(`{"token":"` + proxyPassToken + `"}`))
		default:
			http.NotFound(writer, request)
		}
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &fakeGuardianServer{BaseURL: server.URL, Hostname: fakeGuardianHost, Port: uint16(server.Listener.Addr().(*net.TCPAddr).Port), server: server}
}

func (s *fakeFxAServer) EndpointURL() string {
	return "http://" + net.JoinHostPort(s.Hostname, strconv.Itoa(int(s.Port)))
}

func (s *fakeFxAServer) LoginCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loginCalls
}

func (s *fakeFxAServer) SessionTokens() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.sessionTokens...)
}

func (s *fakeGuardianServer) EndpointURL() string {
	return "http://" + net.JoinHostPort(s.Hostname, strconv.Itoa(int(s.Port)))
}

func newFakeH2ProxyServer(t *testing.T) *fakeH2ProxyServer {
	t.Helper()
	proxy := &fakeH2ProxyServer{ServerName: fakeProxyHost}
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodConnect {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if request.ProtoMajor != 2 {
			writer.WriteHeader(http.StatusHTTPVersionNotSupported)
			return
		}
		if request.Header.Get("Proxy-Authorization") != "Bearer "+proxy.ExpectedProxyPassToken {
			writer.WriteHeader(http.StatusProxyAuthRequired)
			_, _ = writer.Write([]byte("unexpected proxy authorization"))
			return
		}
		proxy.mu.Lock()
		proxy.connectDestinations = append(proxy.connectDestinations, request.Host)
		proxy.mu.Unlock()
		upstream, err := net.Dial("tcp", request.Host)
		if err != nil {
			writer.WriteHeader(http.StatusBadGateway)
			_, _ = writer.Write([]byte(err.Error()))
			return
		}
		defer upstream.Close()
		writer.WriteHeader(http.StatusOK)
		flusher, _ := writer.(http.Flusher)
		if flusher != nil {
			flusher.Flush()
		}
		copyDone := make(chan struct{}, 1)
		go func() {
			_, _ = io.Copy(upstream, request.Body)
			if tcpConn, ok := upstream.(*net.TCPConn); ok {
				_ = tcpConn.CloseWrite()
			}
			copyDone <- struct{}{}
		}()
		_, _ = io.Copy(flushWriter{Writer: writer, Flusher: flusher}, upstream)
		<-copyDone
	})
	server := newFakeHTTP2Server(t, fakeProxyHost, handler)
	proxy.URL = server.URL
	proxy.Port = server.port()
	proxy.CertificatePEM = server.rootCAPEM
	proxy.server = server.server
	return proxy
}

func (s *fakeH2ProxyServer) EndpointAddress() string {
	return net.JoinHostPort(s.ServerName, strconv.Itoa(int(s.Port)))
}

func (s *fakeH2ProxyServer) ConnectDestinations() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.connectDestinations...)
}

func newEchoBackend(t *testing.T) *echoBackend {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	backend := &echoBackend{Address: listener.Addr().String(), Port: uint16(listener.Addr().(*net.TCPAddr).Port), listener: listener}
	backend.wg.Add(1)
	go func() {
		defer backend.wg.Done()
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				if errors.Is(acceptErr, net.ErrClosed) {
					return
				}
				return
			}
			backend.wg.Add(1)
			go func() {
				defer backend.wg.Done()
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()
	t.Cleanup(func() {
		_ = backend.listener.Close()
		backend.wg.Wait()
	})
	return backend
}

func newProxyPassToken(t *testing.T, expiry time.Time) string {
	t.Helper()
	header := encodeJWTPart(t, map[string]string{"alg": "HS256", "typ": "JWT"})
	payload := encodeJWTPart(t, map[string]any{
		"sub": "firefox-vpn-user",
		"aud": "firefox-vpn",
		"iat": time.Now().Unix(),
		"nbf": time.Now().Add(-time.Minute).Unix(),
		"exp": expiry.Unix(),
		"iss": "guardian.test",
	})
	signature := base64.RawURLEncoding.EncodeToString([]byte("test-signature"))
	return strings.Join([]string{header, payload, signature}, ".")
}

func encodeJWTPart(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(encoded)
}
