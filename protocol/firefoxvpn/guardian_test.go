package firefoxvpn

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFirefoxVPNGuardianFetchProxyPass_sendsBearerTokenAndParsesClaims_whenTokenReturned(t *testing.T) {
	t.Parallel()

	jwtToken := newProxyPassJWT(t, ProxyPassClaims{
		Sub: "sub-1",
		Aud: "aud-1",
		Iat: 100,
		Nbf: 200,
		Exp: 300,
		Iss: "guardian",
	})

	var authorizationHeader string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodGet, request.Method)
		require.Equal(t, "/api/v1/fpn/token", request.URL.Path)

		authorizationHeader = request.Header.Get("Authorization")
		writer.Header().Set("X-Quota-Limit", "10")
		writer.Header().Set("X-Quota-Remaining", "9")
		writer.Header().Set("X-Quota-Reset", "12345")
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(writer, `{"token":%q}`, jwtToken)
	}))
	defer server.Close()

	client := newControlPlaneClient(server.Client(), controlPlaneEndpoints{guardianBaseURL: server.URL})

	proxyPass, err := client.FetchProxyPass(context.Background(), "access-token")
	require.NoError(t, err)
	require.Equal(t, "Bearer access-token", authorizationHeader)
	require.Equal(t, jwtToken, proxyPass.Token)
	require.Equal(t, "sub-1", proxyPass.Claims.Sub)
	require.Equal(t, "aud-1", proxyPass.Claims.Aud)
	require.Equal(t, int64(300), proxyPass.Claims.Exp)
	require.Equal(t, "10", proxyPass.QuotaLimit)
	require.Equal(t, "9", proxyPass.QuotaRemaining)
	require.Equal(t, "12345", proxyPass.QuotaReset)
}

func TestFirefoxVPNGuardian_returnsErrorsForHTTPFailureAndEmptyToken_whenUpstreamRejectsRequest(t *testing.T) {
	t.Parallel()

	t.Run("http_failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusTooManyRequests)
			_, _ = writer.Write([]byte(`{"error":"quota"}`))
		}))
		defer server.Close()

		client := newControlPlaneClient(server.Client(), controlPlaneEndpoints{guardianBaseURL: server.URL})
		_, err := client.FetchProxyPass(context.Background(), "access-token")
		require.ErrorContains(t, err, "guardian proxy-pass failed")
	})

	t.Run("empty_token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"token":""}`))
		}))
		defer server.Close()

		client := newControlPlaneClient(server.Client(), controlPlaneEndpoints{guardianBaseURL: server.URL})
		_, err := client.FetchProxyPass(context.Background(), "access-token")
		require.ErrorContains(t, err, "missing token")
	})
}

func newProxyPassJWT(t *testing.T, claims ProxyPassClaims) string {
	t.Helper()

	payload, err := json.Marshal(claims)
	require.NoError(t, err)

	return base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`)) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}
