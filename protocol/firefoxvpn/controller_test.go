package firefoxvpn

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	boxlog "github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/stretchr/testify/require"
)

func TestFirefoxVPNRuntimeAuth_prefersRefreshTokenWhenAccessTokenExpires(t *testing.T) {
	t.Parallel()

	state := newControllerTestState(t)
	controller := state.newController(t)
	controller.now = func() time.Time { return state.now }
	require.NoError(t, controller.Start(t.Context()))

	controller.mu.Lock()
	controller.accessTokenExpiry = state.now.Add(30 * time.Second)
	controller.proxyPass = state.proxyPassInfo("proxy-pass-initial", state.now.Add(2*time.Hour))
	controller.mu.Unlock()

	proxyPass, err := controller.GetProxyPass(t.Context())
	require.NoError(t, err)
	require.Equal(t, "proxy-pass-initial", proxyPass.Token)
	require.Equal(t, 1, state.loginCalls)
	require.Equal(t, 1, state.exchangeCalls)
	require.Equal(t, 1, state.refreshCalls)
	require.Equal(t, 1, state.proxyPassCalls)
	controller.Close()
}

func TestFirefoxVPNProxyPassRenewal_renewsBeforeExpiry(t *testing.T) {
	t.Parallel()

	state := newControllerTestState(t)
	controller := state.newController(t)
	controller.now = func() time.Time { return state.now }
	require.NoError(t, controller.Start(t.Context()))

	controller.mu.Lock()
	controller.accessTokenExpiry = state.now.Add(2 * time.Hour)
	controller.proxyPass = state.proxyPassInfo("proxy-pass-initial", state.now.Add(30*time.Second))
	controller.mu.Unlock()

	proxyPass, err := controller.GetProxyPass(t.Context())
	require.NoError(t, err)
	require.Equal(t, "proxy-pass-refresh-2", proxyPass.Claims.Sub)
	require.Equal(t, 1, state.loginCalls)
	require.Equal(t, 0, state.refreshCalls)
	require.Equal(t, 2, state.proxyPassCalls)
	controller.Close()
}

func TestFirefoxVPNMemoryOnly_doesNotPersistState(t *testing.T) {
	state := newControllerTestState(t)
	controller := state.newController(t)
	controller.now = func() time.Time { return state.now }

	tempDir := t.TempDir()
	oldWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(oldWD))
	})

	require.NoError(t, controller.Start(t.Context()))
	_, err = controller.GetProxyPass(t.Context())
	require.NoError(t, err)

	entries, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	require.Empty(t, entries)

	controller.Close()
	controller.mu.Lock()
	defer controller.mu.Unlock()
	require.Empty(t, controller.password)
	require.Empty(t, controller.accessToken)
	require.Empty(t, controller.refreshToken)
	require.Nil(t, controller.proxyPass)
}

func TestFirefoxVPNBackoff_retriesBoundedly(t *testing.T) {
	t.Parallel()

	state := newControllerTestState(t)
	state.failLogin = true
	controller := state.newController(t)
	controller.retryBaseDelay = 10 * time.Millisecond
	controller.retryMaxDelay = 20 * time.Millisecond
	controller.maxRetries = 2
	controller.operationTimeout = time.Second

	startAt := time.Now()
	err := controller.Start(t.Context())
	require.Error(t, err)
	require.GreaterOrEqual(t, time.Since(startAt), 30*time.Millisecond)
	require.Equal(t, 3, state.loginCalls)
	controller.Close()
}

func TestFirefoxVPNNoSecretLogging_redactsRuntimeFailures(t *testing.T) {
	t.Parallel()

	state := newControllerTestState(t)
	state.failLoginOnce = true
	controller := state.newController(t)
	controller.now = func() time.Time { return state.now }
	controller.logger = newTestLogger(t, &state.logBuffer)
	require.NoError(t, controller.Start(t.Context()))

	logOutput := state.logBuffer.String()
	proxyPassJWT := newProxyPassJWT(t, state.proxyPassInfo("proxy-pass-refresh-1", state.now.Add(2*time.Hour)).Claims)
	for _, secret := range []string{
		state.password,
		"access-secret-token",
		"refresh-secret-token",
		proxyPassJWT,
	} {
		require.NotContains(t, logOutput, secret)
	}
	require.Contains(t, logOutput, "controller retry")
	controller.Close()
}

type controllerTestState struct {
	t              *testing.T
	server         *httptest.Server
	now            time.Time
	password       string
	logBuffer      bytes.Buffer
	mu             sync.Mutex
	loginCalls     int
	exchangeCalls  int
	refreshCalls   int
	proxyPassCalls int
	failLogin      bool
	failLoginOnce  bool
}

func newControllerTestState(t *testing.T) *controllerTestState {
	t.Helper()
	state := &controllerTestState{t: t, now: time.Unix(1_700_000_000, 0), password: "correct horse battery staple"}
	state.server = httptest.NewServer(http.HandlerFunc(state.handle))
	t.Cleanup(state.server.Close)
	return state
}

func (s *controllerTestState) newController(t *testing.T) *AuthController {
	t.Helper()
	controller, err := newAuthControllerWithLogger(t.Context(), boxlog.NewNOPFactory().Logger(), option.FirefoxVPNOutboundOptions{
		ServerOptions: option.ServerOptions{Server: "vpn.example.test", ServerPort: 443},
		Email:         "user@example.com",
		Password:      s.password,
	}, func(context.Context, string) (*ControlPlaneClient, error) {
		return newControlPlaneClient(s.server.Client(), controlPlaneEndpoints{fxaBaseURL: s.server.URL, guardianBaseURL: s.server.URL}), nil
	})
	require.NoError(t, err)
	controller.accessTokenRefreshMargin = time.Minute
	controller.proxyPassRefreshMargin = time.Minute
	return controller
}

func (s *controllerTestState) handle(writer http.ResponseWriter, request *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writer.Header().Set("Content-Type", "application/json")
	switch request.URL.Path {
	case "/account/login":
		s.loginCalls++
		if s.failLogin || (s.failLoginOnce && s.loginCalls == 1) {
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"error":"temporary"}`))
			return
		}
		_, _ = writer.Write([]byte(`{"sessionToken":"abcdef0123456789abcdef0123456789","uid":"uid-1","verified":true,"authAt":123}`))
	case "/oauth/token":
		body, err := io.ReadAll(request.Body)
		require.NoError(s.t, err)
		var tokenRequest FxAOAuthTokenRequest
		require.NoError(s.t, json.Unmarshal(body, &tokenRequest))
		switch tokenRequest.GrantType {
		case "fxa-credentials":
			s.exchangeCalls++
			_, _ = writer.Write([]byte(`{"access_token":"access-secret-token","refresh_token":"refresh-secret-token","expires_in":3600,"scope":"scope","token_type":"bearer"}`))
		case "refresh_token":
			s.refreshCalls++
			_, _ = writer.Write([]byte(`{"access_token":"access-refresh-token","refresh_token":"refresh-secret-token","expires_in":3600,"scope":"scope","token_type":"bearer"}`))
		default:
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(`{"error":"unsupported_grant"}`))
		}
	case "/api/v1/fpn/token":
		s.proxyPassCalls++
		token := "proxy-pass-refresh-1"
		if s.proxyPassCalls > 1 {
			token = "proxy-pass-refresh-2"
		}
		claims := s.proxyPassInfo(token, s.now.Add(2*time.Hour)).Claims
		jwtToken := newProxyPassJWT(s.t, claims)
		_, _ = writer.Write([]byte(`{"token":"` + jwtToken + `"}`))
	default:
		writer.WriteHeader(http.StatusNotFound)
	}
}

func (s *controllerTestState) proxyPassInfo(subject string, expiresAt time.Time) *ProxyPassInfo {
	return &ProxyPassInfo{Token: subject, Claims: ProxyPassClaims{Sub: subject, Aud: "aud", Iat: s.now.Unix(), Nbf: s.now.Unix(), Exp: expiresAt.Unix(), Iss: "guardian"}}
}

func newTestLogger(t *testing.T, writer io.Writer) boxlog.StructuredLogger {
	t.Helper()
	factory, err := boxlog.New(boxlog.Options{Context: t.Context(), Observable: true, DefaultWriter: writer})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, factory.Close())
	})
	return factory.Logger()
}
