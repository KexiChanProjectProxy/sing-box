package firefoxvpn

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	M "github.com/sagernet/sing/common/metadata"
	"github.com/stretchr/testify/require"
)

func TestFirefoxVPNH2Session_reusesSingleUpstreamTLSConnection(t *testing.T) {
	t.Parallel()

	server := newSessionTestServer(t)
	session := server.newSession(t, sessionTestConfig{})
	t.Cleanup(func() {
		require.NoError(t, session.Close())
	})

	require.Equal(t, int32(0), server.connectionCount.Load())

	firstConn, err := session.DialContext(t.Context(), M.ParseSocksaddr("alpha.example:443"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = firstConn.Close() })
	require.NoError(t, roundTripPayload(firstConn, "alpha"))
	require.Equal(t, int32(1), server.connectionCount.Load())

	secondConn, err := session.DialContext(t.Context(), M.ParseSocksaddr("beta.example:443"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = secondConn.Close() })
	require.NoError(t, roundTripPayload(secondConn, "beta"))
	require.Equal(t, int32(1), server.connectionCount.Load())
}

func TestFirefoxVPNConnectTunnel_returnsConnectResponseError_whenProxyRejectsTarget(t *testing.T) {
	t.Parallel()

	server := newSessionTestServer(t)
	server.setStatus("blocked.example:443", http.StatusForbidden)
	session := server.newSession(t, sessionTestConfig{})
	t.Cleanup(func() {
		require.NoError(t, session.Close())
	})

	_, err := session.DialContext(t.Context(), M.ParseSocksaddr("blocked.example:443"))
	require.Error(t, err)
	var connectErr *ConnectResponseError
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, http.StatusForbidden, connectErr.StatusCode)
	require.Contains(t, connectErr.Error(), "403 Forbidden")
}

func TestFirefoxVPNConcurrentStreams_sharesSingleUpstreamConnection_andHonorsClientCap(t *testing.T) {
	t.Parallel()

	t.Run("shares single upstream connection", func(t *testing.T) {
		t.Parallel()

		server := newSessionTestServer(t)
		session := server.newSession(t, sessionTestConfig{})
		t.Cleanup(func() {
			require.NoError(t, session.Close())
		})

		const tunnelCount = 4
		var group sync.WaitGroup
		group.Add(tunnelCount)
		errorsCh := make(chan error, tunnelCount)
		for index := range tunnelCount {
			index := index
			go func() {
				defer group.Done()
				conn, err := session.DialContext(t.Context(), M.ParseSocksaddr(fmt.Sprintf("shared-%d.example:443", index)))
				if err != nil {
					errorsCh <- err
					return
				}
				defer conn.Close()
				errorsCh <- roundTripPayload(conn, fmt.Sprintf("payload-%d", index))
			}()
		}
		group.Wait()
		close(errorsCh)
		for err := range errorsCh {
			require.NoError(t, err)
		}
		require.Equal(t, int32(1), server.connectionCount.Load())
	})

	t.Run("honors client stream cap", func(t *testing.T) {
		t.Parallel()

		server := newSessionTestServer(t)
		session := server.newSession(t, sessionTestConfig{maxConcurrentStreams: 1, streamTimeout: time.Second})
		t.Cleanup(func() {
			require.NoError(t, session.Close())
		})

		firstConn, err := session.DialContext(t.Context(), M.ParseSocksaddr("limited-1.example:443"))
		require.NoError(t, err)
		defer firstConn.Close()

		secondReady := make(chan error, 1)
		go func() {
			secondConn, dialErr := session.DialContext(t.Context(), M.ParseSocksaddr("limited-2.example:443"))
			if dialErr == nil {
				_ = secondConn.Close()
			}
			secondReady <- dialErr
		}()

		select {
		case err := <-secondReady:
			t.Fatalf("second tunnel should wait for stream slot, got %v", err)
		case <-time.After(100 * time.Millisecond):
		}

		require.NoError(t, firstConn.Close())
		require.NoError(t, <-secondReady)
	})
}

func TestFirefoxVPNTimeouts_enforcesDialAndConnectTimeouts(t *testing.T) {
	t.Parallel()

	t.Run("dial timeout", func(t *testing.T) {
		t.Parallel()

		session := newDialTimeoutSession(t, 50*time.Millisecond)
		t.Cleanup(func() {
			require.NoError(t, session.Close())
		})

		_, err := session.DialContext(t.Context(), M.ParseSocksaddr("timeout.example:443"))
		require.Error(t, err)
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("connect timeout", func(t *testing.T) {
		t.Parallel()

		server := newSessionTestServer(t)
		server.setResponseDelay("slow.example:443", 200*time.Millisecond)
		session := server.newSession(t, sessionTestConfig{streamTimeout: 50 * time.Millisecond})
		t.Cleanup(func() {
			require.NoError(t, session.Close())
		})

		_, err := session.DialContext(t.Context(), M.ParseSocksaddr("slow.example:443"))
		require.Error(t, err)
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
}
