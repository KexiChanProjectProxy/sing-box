package hysteria2

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"

	"github.com/sagernet/sing-box/option"
	"github.com/stretchr/testify/require"
)

func TestMasqueradeProxyHTTPS(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("proxied"))
	}))
	client := upstream.Client()
	t.Cleanup(func() {
		client.CloseIdleConnections()
		upstream.Close()
	})

	proxy := newTestMasqueradeProxy(t, upstream.URL, option.Hysteria2MasqueradeProxy{})
	require.Nil(t, proxy.Transport)
	proxy.Transport = client.Transport

	request := httptest.NewRequest(http.MethodGet, "https://requested.example/masquerade", nil)
	request.Host = "requested.example"
	request.RemoteAddr = "198.51.100.7:4242"
	request.TLS = &tls.ConnectionState{}
	response := httptest.NewRecorder()

	proxy.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "proxied", response.Body.String())
}

func TestMasqueradeProxyXForwardedHeaders(t *testing.T) {
	t.Parallel()

	var observedHost string
	var observedHeaders http.Header
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedHost = r.Host
		observedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusNoContent)
	}))
	client := upstream.Client()
	t.Cleanup(func() {
		client.CloseIdleConnections()
		upstream.Close()
	})

	proxy := newTestMasqueradeProxy(t, upstream.URL, option.Hysteria2MasqueradeProxy{
		XForwarded: true,
	})
	proxy.Transport = client.Transport

	request := httptest.NewRequest(http.MethodGet, "https://requested.example/masquerade", nil)
	request.Host = "requested.example"
	request.RemoteAddr = "198.51.100.7:4242"
	request.TLS = &tls.ConnectionState{}
	response := httptest.NewRecorder()

	proxy.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
	require.Equal(t, "requested.example", observedHost)
	require.Equal(t, []string{"198.51.100.7"}, observedHeaders.Values("X-Forwarded-For"))
	require.Equal(t, []string{"requested.example"}, observedHeaders.Values("X-Forwarded-Host"))
	require.Equal(t, []string{"https"}, observedHeaders.Values("X-Forwarded-Proto"))
}

func TestMasqueradeProxyDisablesSpoofedForwardedHeaders(t *testing.T) {
	t.Parallel()

	var observedHeaders http.Header
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusNoContent)
	}))
	client := upstream.Client()
	t.Cleanup(func() {
		client.CloseIdleConnections()
		upstream.Close()
	})

	proxy := newTestMasqueradeProxy(t, upstream.URL, option.Hysteria2MasqueradeProxy{})
	proxy.Transport = client.Transport

	request := httptest.NewRequest(http.MethodGet, "https://requested.example/masquerade", nil)
	request.Host = "requested.example"
	request.Header.Set("X-Forwarded-For", "203.0.113.77")
	request.Header.Set("X-Forwarded-Host", "spoofed.example")
	request.Header.Set("X-Forwarded-Proto", "http")
	response := httptest.NewRecorder()

	proxy.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
	require.Empty(t, observedHeaders.Values("X-Forwarded-For"))
	require.Empty(t, observedHeaders.Values("X-Forwarded-Host"))
	require.Empty(t, observedHeaders.Values("X-Forwarded-Proto"))
}

func TestMasqueradeProxyRewriteHostIndependentOfForwardedHost(t *testing.T) {
	t.Parallel()

	var observedHost string
	var observedForwardedHost []string
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedHost = r.Host
		observedForwardedHost = r.Header.Values("X-Forwarded-Host")
		w.WriteHeader(http.StatusNoContent)
	}))
	client := upstream.Client()
	t.Cleanup(func() {
		client.CloseIdleConnections()
		upstream.Close()
	})

	proxy := newTestMasqueradeProxy(t, upstream.URL, option.Hysteria2MasqueradeProxy{
		RewriteHost: true,
		XForwarded:  true,
	})
	proxy.Transport = client.Transport

	request := httptest.NewRequest(http.MethodGet, "https://requested.example/masquerade", nil)
	request.Host = "requested.example"
	request.RemoteAddr = "198.51.100.7:4242"
	request.TLS = &tls.ConnectionState{}
	response := httptest.NewRecorder()

	proxy.ServeHTTP(response, request)

	target, err := url.Parse(upstream.URL)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, response.Code)
	require.Equal(t, target.Host, observedHost)
	require.Equal(t, []string{"requested.example"}, observedForwardedHost)
}

func TestMasqueradeProxyClosedUpstreamReturns502(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	closedAddress := listener.Addr().String()
	require.NoError(t, listener.Close())

	proxy := newTestMasqueradeProxy(t, "https://"+closedAddress, option.Hysteria2MasqueradeProxy{})

	request := httptest.NewRequest(http.MethodGet, "https://requested.example/masquerade", nil)
	response := httptest.NewRecorder()

	proxy.ServeHTTP(response, request)

	require.Equal(t, http.StatusBadGateway, response.Code)
}

func newTestMasqueradeProxy(t *testing.T, target string, options option.Hysteria2MasqueradeProxy) *httputil.ReverseProxy {
	t.Helper()
	targetURL, err := url.Parse(target)
	require.NoError(t, err)
	return newMasqueradeProxy(targetURL, options)
}
