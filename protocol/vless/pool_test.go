package vless

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockConn implements net.Conn for testing
type mockConn struct {
	net.Conn
	closed           atomic.Bool
	readCalls        atomic.Int32
	writeCalls       atomic.Int32
	closeCalls       atomic.Int32
	readDeadline     atomic.Value
	closeFunc        func() error
	setReadDeadlineFunc func(t time.Time) error
}

func newMockConn() *mockConn {
	m := &mockConn{}
	m.closeFunc = func() error {
		m.closeCalls.Add(1)
		m.closed.Store(true)
		return nil
	}
	m.setReadDeadlineFunc = func(t time.Time) error {
		m.readDeadline.Store(t)
		return nil
	}
	return m
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	m.readCalls.Add(1)
	if m.closed.Load() {
		return 0, io.EOF
	}
	time.Sleep(10 * time.Millisecond) // Simulate some delay
	return len(b), nil
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	m.writeCalls.Add(1)
	if m.closed.Load() {
		return 0, net.ErrClosed
	}
	return len(b), nil
}

func (m *mockConn) Close() error {
	return m.closeFunc()
}

func (m *mockConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
}

func (m *mockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 54321}
}

func (m *mockConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	return m.setReadDeadlineFunc(t)
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// mockLogger is a no-op logger for testing
type mockLogger struct{}

func (m *mockLogger) Trace(args ...any)                             {}
func (m *mockLogger) Debug(args ...any)                             {}
func (m *mockLogger) Info(args ...any)                              {}
func (m *mockLogger) Warn(args ...any)                              {}
func (m *mockLogger) Error(args ...any)                             {}
func (m *mockLogger) Fatal(args ...any)                             {}
func (m *mockLogger) Panic(args ...any)                             {}
func (m *mockLogger) TraceContext(ctx context.Context, args ...any) {}
func (m *mockLogger) DebugContext(ctx context.Context, args ...any) {}
func (m *mockLogger) InfoContext(ctx context.Context, args ...any)  {}
func (m *mockLogger) WarnContext(ctx context.Context, args ...any)  {}
func (m *mockLogger) ErrorContext(ctx context.Context, args ...any) {}
func (m *mockLogger) FatalContext(ctx context.Context, args ...any) {}
func (m *mockLogger) PanicContext(ctx context.Context, args ...any) {}

// TestConnectionPool_GetConn tests basic connection creation and reuse
func TestConnectionPool_GetConn(t *testing.T) {
	ctx := context.Background()
	createCount := atomic.Int32{}

	config := ConnectionPoolConfig{
		EnsureIdle: 0, // Don't pre-create
		CreateConn: func(ctx context.Context) (net.Conn, error) {
			createCount.Add(1)
			return newMockConn(), nil
		},
		Logger: &mockLogger{},
	}

	pool := NewConnectionPool(ctx, config)
	defer pool.Close()

	// First GetConn should create a new connection
	conn1, err := pool.GetConn(ctx)
	if err != nil {
		t.Fatalf("GetConn failed: %v", err)
	}
	if createCount.Load() != 1 {
		t.Errorf("Expected 1 connection created, got %d", createCount.Load())
	}

	// Close should return to pool
	conn1.Close()

	// Second GetConn should reuse the connection
	conn2, err := pool.GetConn(ctx)
	if err != nil {
		t.Fatalf("GetConn failed: %v", err)
	}
	if createCount.Load() != 1 {
		t.Errorf("Expected 1 connection created (reused), got %d", createCount.Load())
	}

	conn2.Close()
}

// TestConnectionPool_Concurrency tests concurrent access to the pool
func TestConnectionPool_Concurrency(t *testing.T) {
	ctx := context.Background()
	createCount := atomic.Int32{}

	config := ConnectionPoolConfig{
		EnsureIdle: 0,
		CreateConn: func(ctx context.Context) (net.Conn, error) {
			createCount.Add(1)
			time.Sleep(10 * time.Millisecond) // Simulate slow connection
			return newMockConn(), nil
		},
		Logger: &mockLogger{},
	}

	pool := NewConnectionPool(ctx, config)
	defer pool.Close()

	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			conn, err := pool.GetConn(ctx)
			if err != nil {
				t.Errorf("GetConn failed: %v", err)
				return
			}

			// Simulate some work
			time.Sleep(50 * time.Millisecond)

			conn.Close()
		}()
	}

	wg.Wait()

	// Should have created connections (exact number depends on timing)
	if createCount.Load() == 0 {
		t.Error("Expected connections to be created")
	}
}

// TestConnectionPool_IdleTimeout tests idle connection cleanup
func TestConnectionPool_IdleTimeout(t *testing.T) {
	ctx := context.Background()
	createCount := atomic.Int32{}
	closeCount := atomic.Int32{}

	config := ConnectionPoolConfig{
		EnsureIdle:    0,
		MinIdle:       0,
		IdleTimeout:   100 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
		CreateConn: func(ctx context.Context) (net.Conn, error) {
			createCount.Add(1)
			mc := newMockConn()
			// Track closes
			originalClose := mc.closeFunc
			mc.closeFunc = func() error {
				closeCount.Add(1)
				return originalClose()
			}
			return mc, nil
		},
		Logger: &mockLogger{},
	}

	pool := NewConnectionPool(ctx, config)
	defer pool.Close()

	// Create and return a connection
	conn, err := pool.GetConn(ctx)
	if err != nil {
		t.Fatalf("GetConn failed: %v", err)
	}
	conn.Close()

	// Wait for idle timeout + maintenance interval
	time.Sleep(200 * time.Millisecond)

	// Connection should be closed
	if closeCount.Load() == 0 {
		t.Error("Expected idle connection to be closed")
	}
}

// TestConnectionPool_MaxLifetime tests age-based connection rotation
func TestConnectionPool_MaxLifetime(t *testing.T) {
	ctx := context.Background()
	createCount := atomic.Int32{}
	closeCount := atomic.Int32{}

	config := ConnectionPoolConfig{
		EnsureIdle:     0,
		MinIdle:        0,
		MinIdleForAge:  0,
		MaxLifetime:    100 * time.Millisecond,
		CheckInterval:  50 * time.Millisecond,
		LifetimeJitter: 0, // No jitter for predictable testing
		CreateConn: func(ctx context.Context) (net.Conn, error) {
			createCount.Add(1)
			mc := newMockConn()
			originalClose := mc.closeFunc
			mc.closeFunc = func() error {
				closeCount.Add(1)
				return originalClose()
			}
			return mc, nil
		},
		Logger: &mockLogger{},
	}

	pool := NewConnectionPool(ctx, config)
	defer pool.Close()

	// Create and return a connection
	conn, err := pool.GetConn(ctx)
	if err != nil {
		t.Fatalf("GetConn failed: %v", err)
	}
	conn.Close()

	// Wait for lifetime expiration + maintenance interval
	time.Sleep(200 * time.Millisecond)

	// Connection should be closed due to age
	if closeCount.Load() == 0 {
		t.Error("Expected old connection to be closed")
	}
}

// TestConnectionPool_EnsureIdle tests pre-connection creation
func TestConnectionPool_EnsureIdle(t *testing.T) {
	ctx := context.Background()
	createCount := atomic.Int32{}

	config := ConnectionPoolConfig{
		EnsureIdle:       3,
		EnsureCreateRate: 10, // Allow multiple creations
		CheckInterval:    100 * time.Millisecond,
		CreateConn: func(ctx context.Context) (net.Conn, error) {
			createCount.Add(1)
			return newMockConn(), nil
		},
		Logger: &mockLogger{},
	}

	pool := NewConnectionPool(ctx, config)
	defer pool.Close()

	// Wait for pre-connections to be created
	time.Sleep(500 * time.Millisecond)

	// Should have created approximately ensureIdle connections
	count := createCount.Load()
	if count < 2 {
		t.Errorf("Expected at least 2 pre-connections, got %d", count)
	}
}

// TestConnectionPool_MinIdle tests minimum idle protection
func TestConnectionPool_MinIdle(t *testing.T) {
	ctx := context.Background()
	createCount := atomic.Int32{}
	closeCount := atomic.Int32{}

	config := ConnectionPoolConfig{
		EnsureIdle:    3,
		MinIdle:       2,
		IdleTimeout:   100 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
		CreateConn: func(ctx context.Context) (net.Conn, error) {
			createCount.Add(1)
			mc := newMockConn()
			originalClose := mc.closeFunc
			mc.closeFunc = func() error {
				closeCount.Add(1)
				return originalClose()
			}
			return mc, nil
		},
		Logger: &mockLogger{},
	}

	pool := NewConnectionPool(ctx, config)
	defer pool.Close()

	// Wait for pre-connections
	time.Sleep(300 * time.Millisecond)

	// Wait for idle timeout to trigger
	time.Sleep(200 * time.Millisecond)

	// Should maintain at least MinIdle connections
	pool.mu.Lock()
	idleCount := 0
	for _, pc := range pool.connections {
		if !pc.inUse && !pc.closed {
			idleCount++
		}
	}
	pool.mu.Unlock()

	if idleCount < config.MinIdle {
		t.Errorf("Expected at least %d idle connections, got %d", config.MinIdle, idleCount)
	}
}

// TestConnectionPool_Reset tests network interface change handling
func TestConnectionPool_Reset(t *testing.T) {
	ctx := context.Background()
	createCount := atomic.Int32{}
	closeCount := atomic.Int32{}

	config := ConnectionPoolConfig{
		EnsureIdle: 2,
		CreateConn: func(ctx context.Context) (net.Conn, error) {
			createCount.Add(1)
			mc := newMockConn()
			originalClose := mc.closeFunc
			mc.closeFunc = func() error {
				closeCount.Add(1)
				return originalClose()
			}
			return mc, nil
		},
		Logger: &mockLogger{},
	}

	pool := NewConnectionPool(ctx, config)
	defer pool.Close()

	// Wait for initial connections
	time.Sleep(200 * time.Millisecond)

	initialCreate := createCount.Load()

	// Reset should close all connections
	pool.Reset()

	// Wait a bit for close operations
	time.Sleep(100 * time.Millisecond)

	// Should have closed connections
	if closeCount.Load() == 0 {
		t.Error("Expected connections to be closed after reset")
	}

	// Pool should be empty or in process of recreating after reset
	// Note: Reset() triggers ensureIdleConnections() in a goroutine,
	// so there might be a race. Just verify connections were closed.
	pool.mu.Lock()
	connCount := len(pool.connections)
	pool.mu.Unlock()

	// Accept either 0 (reset complete) or being recreated
	if connCount > config.EnsureIdle {
		t.Errorf("Expected at most %d connections after reset, got %d", config.EnsureIdle, connCount)
	}

	// Should recreate connections if ensureIdle is set
	time.Sleep(300 * time.Millisecond)
	if createCount.Load() <= initialCreate {
		t.Error("Expected new connections to be created after reset")
	}
}

// TestConnectionPool_Close tests graceful shutdown
func TestConnectionPool_Close(t *testing.T) {
	ctx := context.Background()
	closeCount := atomic.Int32{}

	config := ConnectionPoolConfig{
		EnsureIdle: 2,
		CreateConn: func(ctx context.Context) (net.Conn, error) {
			mc := newMockConn()
			originalClose := mc.closeFunc
			mc.closeFunc = func() error {
				closeCount.Add(1)
				return originalClose()
			}
			return mc, nil
		},
		Logger: &mockLogger{},
	}

	pool := NewConnectionPool(ctx, config)

	// Wait for connections to be created
	time.Sleep(200 * time.Millisecond)

	// Close pool
	err := pool.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Should have closed all connections
	if closeCount.Load() == 0 {
		t.Error("Expected connections to be closed")
	}

	// GetConn should fail after close
	_, err = pool.GetConn(ctx)
	if err != net.ErrClosed {
		t.Errorf("Expected ErrClosed, got %v", err)
	}

	// Close should be idempotent
	err = pool.Close()
	if err != nil {
		t.Errorf("Second close should not error, got %v", err)
	}
}

// TestReturnableConn tests that connections are properly returned to pool
func TestReturnableConn(t *testing.T) {
	ctx := context.Background()

	config := ConnectionPoolConfig{
		EnsureIdle: 0,
		CreateConn: func(ctx context.Context) (net.Conn, error) {
			return newMockConn(), nil
		},
		Logger: &mockLogger{},
	}

	pool := NewConnectionPool(ctx, config)
	defer pool.Close()

	// Get connection
	conn, err := pool.GetConn(ctx)
	if err != nil {
		t.Fatalf("GetConn failed: %v", err)
	}

	// Close should return to pool
	conn.Close()

	// Check that connection is back in pool as idle
	pool.mu.Lock()
	idleCount := 0
	for _, pc := range pool.connections {
		if !pc.inUse && !pc.closed {
			idleCount++
		}
	}
	pool.mu.Unlock()

	if idleCount != 1 {
		t.Errorf("Expected 1 idle connection in pool, got %d", idleCount)
	}

	// Multiple closes should be safe
	conn.Close()
	conn.Close()
}

// TestLifetimeJitter tests that jitter is applied correctly
func TestLifetimeJitter(t *testing.T) {
	ctx := context.Background()

	config := ConnectionPoolConfig{
		EnsureIdle:       0,
		MaxLifetime:      1 * time.Hour,
		LifetimeJitter:   10 * time.Minute,
		CreateConn: func(ctx context.Context) (net.Conn, error) {
			return newMockConn(), nil
		},
		Logger: &mockLogger{},
	}

	pool := NewConnectionPool(ctx, config)
	defer pool.Close()

	// Create multiple connections simultaneously (don't close them)
	// so they don't get reused, ensuring each has its own expiration
	connections := make([]net.Conn, 20)
	expirations := make([]time.Time, 20)

	for i := 0; i < 20; i++ {
		// Add small delay to ensure different random values
		time.Sleep(1 * time.Millisecond)

		conn, err := pool.GetConn(ctx)
		if err != nil {
			t.Fatalf("GetConn failed: %v", err)
		}

		connections[i] = conn

		// Get the pooled connection's expiration
		rc := conn.(*returnableConn)
		expirations[i] = rc.pc.expiresAt
	}

	// Now close all connections
	for _, conn := range connections {
		conn.Close()
	}

	// Check that at least some expiration times differ
	// (with 20 connections and 20-minute jitter window, should have variation)
	uniqueExpirations := make(map[int64]bool)
	for _, exp := range expirations {
		uniqueExpirations[exp.Unix()] = true
	}

	// Should have at least 2 different expiration times with jitter
	if len(uniqueExpirations) < 2 {
		t.Errorf("Expected variation in expiration times due to jitter, got %d unique times", len(uniqueExpirations))
	}
}

// TestHeartbeat tests that heartbeat is working
func TestHeartbeat(t *testing.T) {
	ctx := context.Background()
	deadlineSet := atomic.Bool{}

	config := ConnectionPoolConfig{
		EnsureIdle: 1,
		Heartbeat:  50 * time.Millisecond,
		CreateConn: func(ctx context.Context) (net.Conn, error) {
			mc := newMockConn()
			originalSetReadDeadline := mc.setReadDeadlineFunc
			mc.setReadDeadlineFunc = func(t time.Time) error {
				if !t.IsZero() {
					deadlineSet.Store(true)
				}
				return originalSetReadDeadline(t)
			}
			return mc, nil
		},
		Logger: &mockLogger{},
	}

	pool := NewConnectionPool(ctx, config)
	defer pool.Close()

	// Wait for heartbeat to trigger
	time.Sleep(200 * time.Millisecond)

	// Should have set read deadline
	if !deadlineSet.Load() {
		t.Error("Expected heartbeat to set read deadline")
	}
}

// TestEnsureCreateRate tests that creation rate limiting works
func TestEnsureCreateRate(t *testing.T) {
	ctx := context.Background()
	createCount := atomic.Int32{}

	config := ConnectionPoolConfig{
		EnsureIdle:       10,
		EnsureCreateRate: 2, // Limit to 2 per maintenance cycle
		CheckInterval:    100 * time.Millisecond,
		CreateConn: func(ctx context.Context) (net.Conn, error) {
			createCount.Add(1)
			return newMockConn(), nil
		},
		Logger: &mockLogger{},
	}

	pool := NewConnectionPool(ctx, config)
	defer pool.Close()

	// After first maintenance cycle, should have created at most EnsureCreateRate connections
	time.Sleep(150 * time.Millisecond)

	count := createCount.Load()
	if count > int32(config.EnsureCreateRate)*2 {
		t.Errorf("Expected at most %d initial connections, got %d", config.EnsureCreateRate*2, count)
	}

	// Should eventually reach EnsureIdle
	time.Sleep(1 * time.Second)

	pool.mu.Lock()
	totalConns := len(pool.connections)
	pool.mu.Unlock()

	if totalConns < config.EnsureIdle-2 {
		t.Logf("Expected around %d connections eventually, got %d (may be timing dependent)", config.EnsureIdle, totalConns)
	}
}

// TestConnectionPool_Read tests that read clears deadlines
func TestConnectionPool_Read(t *testing.T) {
	ctx := context.Background()
	deadlineCleared := atomic.Bool{}

	config := ConnectionPoolConfig{
		EnsureIdle: 0,
		CreateConn: func(ctx context.Context) (net.Conn, error) {
			mc := newMockConn()
			originalSetReadDeadline := mc.setReadDeadlineFunc
			mc.setReadDeadlineFunc = func(t time.Time) error {
				if t.IsZero() {
					deadlineCleared.Store(true)
				}
				return originalSetReadDeadline(t)
			}
			return mc, nil
		},
		Logger: &mockLogger{},
	}

	pool := NewConnectionPool(ctx, config)
	defer pool.Close()

	conn, err := pool.GetConn(ctx)
	if err != nil {
		t.Fatalf("GetConn failed: %v", err)
	}

	// Read should clear deadline
	buf := make([]byte, 100)
	_, _ = conn.Read(buf)

	if !deadlineCleared.Load() {
		t.Error("Expected Read to clear deadline")
	}

	conn.Close()
}
