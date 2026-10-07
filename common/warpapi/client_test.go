package warpapi

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing/common/json"

	"github.com/stretchr/testify/require"
)

func TestRegisterFlow(t *testing.T) {
	t.Parallel()

	serverKey, err := GeneratePrivateKey()
	require.NoError(t, err)
	serverPublicDER, err := x509.MarshalPKIXPublicKey(&serverKey.PublicKey)
	require.NoError(t, err)
	serverPublicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: serverPublicDER}))

	clientKey, err := GeneratePrivateKey()
	require.NoError(t, err)
	var licenseCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "WARP for Android", request.Header.Get("User-Agent"))
		require.Equal(t, "a-6.35-4471", request.Header.Get("CF-Client-Version"))
		body, _ := io.ReadAll(request.Body)
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/reg":
			require.Equal(t, "jwt-value", request.Header.Get("CF-Access-Jwt-Assertion"))
			require.Empty(t, request.Header.Get("Authorization"))
			var registerBody map[string]any
			require.NoError(t, json.Unmarshal(body, &registerBody))
			require.Equal(t, "curve25519", registerBody["key_type"])
			require.Equal(t, "wireguard", registerBody["tunnel_type"])
			require.Equal(t, "PC", registerBody["model"])
			require.Equal(t, "en_US", registerBody["locale"])
			require.Equal(t, "2026-10-07T12:00:00.000+00:00", registerBody["tos"])
			require.Len(t, registerBody["serial_number"], 16)
			writer.Write([]byte(`{"id":"device-1","token":"secret-token","account":{"id":"account-1"},"config":{"peers":[]}}`))
		case request.Method == http.MethodPatch && request.URL.Path == "/reg/device-1":
			require.Equal(t, "Bearer secret-token", request.Header.Get("Authorization"))
			var update deviceUpdateRequest
			require.NoError(t, json.Unmarshal(body, &update))
			require.Equal(t, "secp256r1", update.KeyType)
			require.Equal(t, "masque", update.TunnelType)
			require.Equal(t, "my-box", update.Name)
			publicKey, err := ParsePublicKey(update.Key)
			require.NoError(t, err)
			require.True(t, publicKey.Equal(&clientKey.PublicKey))
			response := map[string]any{
				"id":      "device-1",
				"account": map[string]any{"id": "account-1"},
				"config": map[string]any{
					"peers": []any{map[string]any{
						"public_key": serverPublicPEM,
						"endpoint":   map[string]any{"v4": "162.159.198.1:0", "v6": "[2606:4700:103::]:0"},
					}},
					"interface": map[string]any{"addresses": map[string]any{"v4": "172.16.0.2", "v6": "2606:4700:110:8a36::1"}},
				},
			}
			json.NewEncoder(writer).Encode(response)
		case request.Method == http.MethodPut && request.URL.Path == "/reg/device-1/account":
			licenseCalls.Add(1)
			require.Equal(t, "Bearer secret-token", request.Header.Get("Authorization"))
			require.JSONEq(t, `{"license":"LICENSE-1"}`, string(body))
			writer.Write([]byte(`{"id":"account-1","license":"LICENSE-1","warp_plus":true}`))
		default:
			t.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client(), server.URL)
	client.now = func() time.Time { return time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC) }
	registration, err := client.Register(context.Background(), clientKey, RegisterOptions{
		Name:      "my-box",
		AccessJWT: "jwt-value",
		License:   "LICENSE-1",
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), licenseCalls.Load())
	require.Equal(t, "device-1", registration.DeviceID)
	require.Equal(t, "secret-token", registration.AccessToken)
	require.Equal(t, "LICENSE-1", registration.License)
	require.Equal(t, netip.MustParseAddr("162.159.198.1"), registration.EndpointV4)
	require.Equal(t, netip.MustParseAddr("2606:4700:103::"), registration.EndpointV6)
	require.Equal(t, base64.StdEncoding.EncodeToString(serverPublicDER), registration.EndpointPublicKey)
	require.Equal(t, []netip.Prefix{
		netip.MustParsePrefix("172.16.0.2/32"),
		netip.MustParsePrefix("2606:4700:110:8a36::1/128"),
	}, registration.Addresses())

	parsedKey, err := ParsePrivateKey(registration.PrivateKey)
	require.NoError(t, err)
	require.True(t, parsedKey.Equal(clientKey))

	data, err := json.Marshal(registration)
	require.NoError(t, err)
	var decoded Registration
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, *registration, decoded)

	redacted := registration.Redacted()
	require.NotContains(t, redacted.AccessToken, "secret")
	require.Equal(t, "secret-token", registration.AccessToken)
}

func TestAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		writer.Write([]byte(`{"success":false,"errors":[{"code":1001,"message":"Invalid public key"}]}`))
	}))
	defer server.Close()

	key, err := GeneratePrivateKey()
	require.NoError(t, err)
	client := NewClient(server.Client(), server.URL)
	_, err = client.Enroll(context.Background(), "device", "secret-token", key, "")
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	require.Equal(t, CodeInvalidPublicKey, apiErr.Code)
	require.Equal(t, http.StatusBadRequest, apiErr.Status)
	require.NotContains(t, err.Error(), "secret-token")
}

func TestKeyFormats(t *testing.T) {
	t.Parallel()

	key, err := GeneratePrivateKey()
	require.NoError(t, err)

	sec1, err := EncodePrivateKey(key)
	require.NoError(t, err)
	pkcs8DER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	sec1DER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	for _, content := range []string{
		sec1,
		base64.StdEncoding.EncodeToString(pkcs8DER),
		string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: sec1DER})),
		string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8DER})),
	} {
		parsed, err := ParsePrivateKey(content)
		require.NoError(t, err)
		require.True(t, parsed.Equal(key))
	}
	_, err = ParsePrivateKey("not base64!")
	require.Error(t, err)

	addr, err := NormalizeEndpoint("162.159.198.1")
	require.NoError(t, err)
	require.Equal(t, netip.MustParseAddr("162.159.198.1"), addr)
}
