package firefoxvpn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFirefoxVPNControlPlaneClient_usesSystemRootTLSAndInjectedDefaults_whenConstructed(t *testing.T) {
	t.Parallel()

	client, err := NewControlPlaneHTTPClient(context.Background(), "")
	require.NoError(t, err)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.NotNil(t, transport.TLSClientConfig)
	require.Nil(t, transport.TLSClientConfig.RootCAs)

	controlClient := newControlPlaneClient(client, defaultControlPlaneEndpoints())
	require.Equal(t, defaultFxABaseURL, controlClient.endpoints.fxaBaseURL)
	require.Equal(t, defaultGuardianBaseURL, controlClient.endpoints.guardianBaseURL)
}

func TestFirefoxVPNFxaLogin_sendsDerivedAuthPW_whenCredentialsValid(t *testing.T) {
	t.Parallel()

	var capturedRequest FxALoginRequest
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodPost, request.Method)
		require.Equal(t, "/account/login", request.URL.Path)

		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &capturedRequest))

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"sessionToken":"abcdef0123456789abcdef0123456789","uid":"uid-1","verified":true,"authAt":123}`))
	}))
	defer server.Close()

	client := newControlPlaneClient(server.Client(), ControlPlaneEndpoints{fxaBaseURL: server.URL})

	response, err := client.Login(context.Background(), "user@example.com", "correct horse battery staple")
	require.NoError(t, err)
	require.Equal(t, "abcdef0123456789abcdef0123456789", response.SessionToken)
	require.Equal(t, "user@example.com", capturedRequest.Email)
	require.Len(t, capturedRequest.AuthPW, derivedKeyLength*2)
	require.NotContains(t, capturedRequest.AuthPW, "-")
}

func TestFirefoxVPNFxaOAuthToken_sendsHawkAuthorization_whenSessionTokenValid(t *testing.T) {
	t.Parallel()

	var authorizationHeader string
	var capturedRequest FxAOAuthTokenRequest
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodPost, request.Method)
		require.Equal(t, "/oauth/token", request.URL.Path)

		authorizationHeader = request.Header.Get("Authorization")
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &capturedRequest))

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"access_token":"access-token","refresh_token":"refresh-token","expires_in":3600,"scope":"scope","token_type":"bearer"}`))
	}))
	defer server.Close()

	client := newControlPlaneClient(server.Client(), ControlPlaneEndpoints{fxaBaseURL: server.URL})
	client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	client.nonce = strings.NewReader("ABCDEF")

	response, err := client.ExchangeOAuthToken(context.Background(), strings.Repeat("a1", 16))
	require.NoError(t, err)
	require.Equal(t, "access-token", response.AccessToken)
	require.Equal(t, "refresh-token", response.RefreshToken)
	require.Equal(t, fxaClientID, capturedRequest.ClientID)
	require.Equal(t, "fxa-credentials", capturedRequest.GrantType)
	require.Equal(t, fxaOAuthScope, capturedRequest.Scope)
	require.Contains(t, authorizationHeader, `Hawk id="`)
	require.Contains(t, authorizationHeader, `hash="`)
}

func TestFirefoxVPNRefreshToken_sendsRefreshGrant_whenRefreshTokenValid(t *testing.T) {
	t.Parallel()

	var capturedRequest FxAOAuthTokenRequest
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodPost, request.Method)
		require.Equal(t, "/oauth/token", request.URL.Path)
		require.Empty(t, request.Header.Get("Authorization"))

		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &capturedRequest))

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"access_token":"new-access-token","expires_in":1800,"scope":"scope","token_type":"bearer"}`))
	}))
	defer server.Close()

	client := newControlPlaneClient(server.Client(), ControlPlaneEndpoints{fxaBaseURL: server.URL})

	response, err := client.RefreshOAuthToken(context.Background(), "refresh-token")
	require.NoError(t, err)
	require.Equal(t, "new-access-token", response.AccessToken)
	require.Equal(t, "refresh-token", response.RefreshToken)
	require.Equal(t, fxaClientID, capturedRequest.ClientID)
	require.Equal(t, "refresh_token", capturedRequest.GrantType)
	require.Equal(t, "refresh-token", capturedRequest.RefreshToken)
}

func TestFirefoxVPNFxa_returnsErrorsForHTTPFailureAndEmptyTokens_whenUpstreamRejectsRequest(t *testing.T) {
	t.Parallel()

	t.Run("login_http_failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			http.Error(writer, "denied", http.StatusUnauthorized)
		}))
		defer server.Close()

		client := newControlPlaneClient(server.Client(), ControlPlaneEndpoints{fxaBaseURL: server.URL})
		_, err := client.Login(context.Background(), "user@example.com", "password")
		require.ErrorContains(t, err, "login failed")
	})

	t.Run("oauth_empty_tokens", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"access_token":"","refresh_token":"","expires_in":3600}`))
		}))
		defer server.Close()

		client := newControlPlaneClient(server.Client(), ControlPlaneEndpoints{fxaBaseURL: server.URL})
		client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
		client.nonce = strings.NewReader("ABCDEF")

		_, err := client.ExchangeOAuthToken(context.Background(), strings.Repeat("a1", 16))
		require.ErrorContains(t, err, "missing access token")
	})

	t.Run("refresh_http_failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(writer, `{"error":"invalid_grant"}`)
		}))
		defer server.Close()

		client := newControlPlaneClient(server.Client(), ControlPlaneEndpoints{fxaBaseURL: server.URL})
		_, err := client.RefreshOAuthToken(context.Background(), "refresh-token")
		require.ErrorContains(t, err, "refresh failed")
	})
}
