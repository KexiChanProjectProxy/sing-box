package vless

import (
	"context"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/sagernet/sing/common/logger"
)

// PooledConnection wraps a VLESS connection with metadata
type PooledConnection struct {
	conn          net.Conn
	createdAt     time.Time
	lastUsedAt    time.Time
	expiresAt     time.Time // Lifetime deadline (with jitter)
	inUse         bool
	heartbeatDone chan struct{}
	closed        bool
}

// ConnectionPoolConfig contains configuration for the connection pool
type ConnectionPoolConfig struct {
	EnsureIdle       int
	EnsureCreateRate int
	MinIdle          int
	MinIdleForAge    int
	CheckInterval    time.Duration
	IdleTimeout      time.Duration
	MaxLifetime      time.Duration
	LifetimeJitter   time.Duration
	Heartbeat        time.Duration
	CreateConn       func(ctx context.Context) (net.Conn, error)
	Logger           logger.ContextLogger
}

// ConnectionPool manages pre-established VLESS connections
type ConnectionPool struct {
	// Config
	ensureIdle       int
	ensureCreateRate int
	minIdle          int
	minIdleForAge    int
	checkInterval    time.Duration
	idleTimeout      time.Duration
	maxLifetime      time.Duration
	lifetimeJitter   time.Duration
	heartbeat        time.Duration

	// Factory function
	createConn func(ctx context.Context) (net.Conn, error)

	// State
	mu          sync.Mutex
	connections []*PooledConnection

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	ticker *time.Ticker
	closed bool

	logger logger.ContextLogger
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(ctx context.Context, config ConnectionPoolConfig) *ConnectionPool {
	poolCtx, cancel := context.WithCancel(ctx)

	pool := &ConnectionPool{
		ensureIdle:       config.EnsureIdle,
		ensureCreateRate: config.EnsureCreateRate,
		minIdle:          config.MinIdle,
		minIdleForAge:    config.MinIdleForAge,
		checkInterval:    config.CheckInterval,
		idleTimeout:      config.IdleTimeout,
		maxLifetime:      config.MaxLifetime,
		lifetimeJitter:   config.LifetimeJitter,
		heartbeat:        config.Heartbeat,
		createConn:       config.CreateConn,
		ctx:              poolCtx,
		cancel:           cancel,
		logger:           config.Logger,
		connections:      make([]*PooledConnection, 0),
	}

	// Set default values if not configured
	if pool.checkInterval == 0 {
		pool.checkInterval = 30 * time.Second
	}
	if pool.idleTimeout == 0 {
		pool.idleTimeout = 5 * time.Minute
	}
	if pool.ensureCreateRate == 0 {
		pool.ensureCreateRate = 1
	}

	// Start maintenance loop
	if pool.checkInterval > 0 {
		pool.ticker = time.NewTicker(pool.checkInterval)
		go pool.maintenanceLoop()
	}

	// Pre-warm pool if configured
	if pool.ensureIdle > 0 {
		go pool.ensureIdleConnections()
	}

	return pool
}

// GetConn retrieves a connection from the pool or creates a new one
func (p *ConnectionPool) GetConn(ctx context.Context) (net.Conn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, net.ErrClosed
	}

	// Try to find an idle connection
	for i := 0; i < len(p.connections); i++ {
		pc := p.connections[i]
		if !pc.inUse && !pc.closed {
			// Check if connection has expired
			if p.maxLifetime > 0 && time.Now().After(pc.expiresAt) {
				p.closePooledConnection(pc, i)
				i--
				continue
			}

			// Mark as in use
			pc.inUse = true
			pc.lastUsedAt = time.Now()

			// Return wrapped connection that will return to pool on close
			return &returnableConn{
				Conn: pc.conn,
				pool: p,
				pc:   pc,
			}, nil
		}
	}

	// No idle connection available, create new one
	pc, err := p.createPooledConnection(ctx)
	if err != nil {
		return nil, err
	}

	pc.inUse = true
	pc.lastUsedAt = time.Now()
	p.connections = append(p.connections, pc)

	return &returnableConn{
		Conn: pc.conn,
		pool: p,
		pc:   pc,
	}, nil
}

// returnConn returns a connection to the pool
func (p *ConnectionPool) returnConn(pc *PooledConnection) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed || pc.closed {
		return
	}

	pc.inUse = false
	pc.lastUsedAt = time.Now()
}

// createPooledConnection creates a new connection with metadata
func (p *ConnectionPool) createPooledConnection(ctx context.Context) (*PooledConnection, error) {
	conn, err := p.createConn(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	pc := &PooledConnection{
		conn:          conn,
		createdAt:     now,
		lastUsedAt:    now,
		expiresAt:     p.calculateExpiration(now),
		inUse:         false,
		heartbeatDone: make(chan struct{}),
		closed:        false,
	}

	// Start heartbeat if configured
	if p.heartbeat > 0 {
		go p.heartbeatLoop(pc)
	}

	return pc, nil
}

// calculateExpiration calculates the expiration time with jitter
func (p *ConnectionPool) calculateExpiration(createdAt time.Time) time.Time {
	if p.maxLifetime == 0 {
		return time.Time{} // Zero value means no expiration
	}

	lifetime := p.maxLifetime
	if p.lifetimeJitter > 0 {
		// Add random jitter: [-jitter, +jitter]
		jitter := time.Duration(rand.Int63n(int64(p.lifetimeJitter)*2) - int64(p.lifetimeJitter))
		lifetime += jitter
	}

	return createdAt.Add(lifetime)
}

// ensureIdleConnections creates connections to meet ensure_idle_session target
func (p *ConnectionPool) ensureIdleConnections() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed || p.ensureIdle == 0 {
		return
	}

	idleCount := 0
	for _, pc := range p.connections {
		if !pc.inUse && !pc.closed {
			idleCount++
		}
	}

	needed := p.ensureIdle - idleCount
	if needed <= 0 {
		return
	}

	// Limit creation rate
	if needed > p.ensureCreateRate {
		needed = p.ensureCreateRate
	}

	// Create needed connections in background
	for i := 0; i < needed; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(p.ctx, 30*time.Second)
			defer cancel()

			pc, err := p.createPooledConnection(ctx)
			if err != nil {
				if p.logger != nil {
					p.logger.Warn("failed to create pool connection: ", err)
				}
				return
			}

			p.mu.Lock()
			defer p.mu.Unlock()

			if !p.closed {
				p.connections = append(p.connections, pc)
			} else {
				pc.conn.Close()
			}
		}()
	}
}

// performMaintenance performs cleanup and ensures idle connections
func (p *ConnectionPool) performMaintenance() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	now := time.Now()
	idleCount := 0

	// Phase 1: Cleanup
	for i := 0; i < len(p.connections); i++ {
		pc := p.connections[i]

		if pc.closed {
			p.connections = append(p.connections[:i], p.connections[i+1:]...)
			i--
			continue
		}

		if !pc.inUse {
			idleCount++

			// Check idle timeout
			if p.idleTimeout > 0 && now.Sub(pc.lastUsedAt) > p.idleTimeout {
				// Respect min_idle_session
				if idleCount > p.minIdle {
					p.closePooledConnection(pc, i)
					i--
					idleCount--
					continue
				}
			}

			// Check age-based expiration
			if p.maxLifetime > 0 && now.After(pc.expiresAt) {
				// Respect min_idle_session_for_age
				minForAge := p.minIdleForAge
				if minForAge == 0 {
					minForAge = p.minIdle
				}
				if idleCount > minForAge {
					p.closePooledConnection(pc, i)
					i--
					idleCount--
					continue
				}
			}
		}
	}

	// Phase 2: Ensure idle connections
	p.mu.Unlock()
	p.ensureIdleConnections()
	p.mu.Lock()
}

// closePooledConnection closes a pooled connection and removes it from the slice
func (p *ConnectionPool) closePooledConnection(pc *PooledConnection, index int) {
	if pc.closed {
		return
	}

	pc.closed = true
	close(pc.heartbeatDone)
	pc.conn.Close()

	if index >= 0 && index < len(p.connections) {
		p.connections = append(p.connections[:index], p.connections[index+1:]...)
	}
}

// maintenanceLoop runs periodic maintenance
func (p *ConnectionPool) maintenanceLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.ticker.C:
			p.performMaintenance()
		}
	}
}

// heartbeatLoop keeps the connection alive with TCP-level keepalive
func (p *ConnectionPool) heartbeatLoop(pc *PooledConnection) {
	ticker := time.NewTicker(p.heartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-pc.heartbeatDone:
			return
		case <-ticker.C:
			// Set read deadline to detect dead connections
			// This is TCP-level keepalive, not VLESS protocol heartbeat
			// Check closed status with lock to avoid race
			p.mu.Lock()
			closed := pc.closed
			p.mu.Unlock()

			if !closed {
				pc.conn.SetReadDeadline(time.Now().Add(p.heartbeat * 2))
			}
		}
	}
}

// Reset closes all connections (for network interface changes)
func (p *ConnectionPool) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := len(p.connections) - 1; i >= 0; i-- {
		p.closePooledConnection(p.connections[i], i)
	}

	// Re-warm pool
	if p.ensureIdle > 0 {
		go p.ensureIdleConnections()
	}
}

// Close shuts down the pool
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	p.cancel()

	if p.ticker != nil {
		p.ticker.Stop()
	}

	for i := len(p.connections) - 1; i >= 0; i-- {
		p.closePooledConnection(p.connections[i], i)
	}

	return nil
}

// returnableConn intercepts Close() to return connection to pool
type returnableConn struct {
	net.Conn
	pool *ConnectionPool
	pc   *PooledConnection
	once sync.Once
}

func (r *returnableConn) Close() error {
	r.once.Do(func() {
		r.pool.returnConn(r.pc)
	})
	return nil
}

// Read overrides to clear read deadlines set by heartbeat
func (r *returnableConn) Read(b []byte) (int, error) {
	// Clear any read deadline set by heartbeat
	r.Conn.SetReadDeadline(time.Time{})
	return r.Conn.Read(b)
}
