package v2rayxhttp

import (
	"context"
	"crypto/rand"
	"math"
	"math/big"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/logger"
)

// XmuxConn represents a connection that can be checked for closed status
type XmuxConn interface {
	IsClosed() bool
}

// XmuxClient represents a single connection in the Xmux pool with usage tracking
type XmuxClient struct {
	httpClient   *http.Client
	OpenUsage    atomic.Int32  // Current concurrent usage count
	leftUsage    int32         // Remaining reuses (-1 = unlimited)
	LeftRequests atomic.Int32  // Remaining HTTP requests
	UnreusableAt time.Time     // Time when connection becomes unusable
	closed       atomic.Bool   // Whether the connection is closed
}

func (c *XmuxClient) IsClosed() bool {
	return c.closed.Load()
}

func (c *XmuxClient) Close() {
	c.closed.Store(true)
	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
}

// XmuxManager manages a pool of XmuxClients for connection reuse
type XmuxManager struct {
	config      *option.V2RayXHTTPXmuxConfig
	logger      logger.ContextLogger
	concurrency int32
	connections int32
	newConnFunc func() *XmuxClient
	xmuxClients []*XmuxClient
}

// NewXmuxManager creates a new connection pool manager
func NewXmuxManager(config *option.V2RayXHTTPXmuxConfig, logger logger.ContextLogger, newConnFunc func() *XmuxClient) *XmuxManager {
	manager := &XmuxManager{
		config:      config,
		logger:      logger,
		newConnFunc: newConnFunc,
		xmuxClients: make([]*XmuxClient, 0),
	}

	// Set concurrency limit (0 = unlimited)
	manager.concurrency = getNormalizedValue(config.MaxConcurrency)
	// Set connection pool size (0 = unlimited)
	manager.connections = getNormalizedValue(config.MaxConnections)

	return manager
}

// newXmuxClient creates a new XmuxClient with configured limits
func (m *XmuxManager) newXmuxClient() *XmuxClient {
	xmuxClient := m.newConnFunc()

	// Set reuse limit
	if x := getNormalizedValue(m.config.CMaxReuseTimes); x > 0 {
		xmuxClient.leftUsage = x - 1
	} else {
		xmuxClient.leftUsage = -1 // unlimited
	}

	// Set request limit
	xmuxClient.LeftRequests.Store(math.MaxInt32)
	if x := getNormalizedValue(m.config.HMaxRequestTimes); x > 0 {
		xmuxClient.LeftRequests.Store(x)
	}

	// Set TTL
	if x := getNormalizedValue(m.config.HMaxReusableSecs); x > 0 {
		xmuxClient.UnreusableAt = time.Now().Add(time.Duration(x) * time.Second)
	}

	m.xmuxClients = append(m.xmuxClients, xmuxClient)
	return xmuxClient
}

// GetXmuxClient retrieves or creates an XmuxClient from the pool
func (m *XmuxManager) GetXmuxClient(ctx context.Context) *XmuxClient {
	// Clean up invalid connections
	for i := 0; i < len(m.xmuxClients); {
		xmuxClient := m.xmuxClients[i]

		// Check if connection should be removed
		if xmuxClient.IsClosed() ||
			xmuxClient.leftUsage == 0 ||
			xmuxClient.LeftRequests.Load() <= 0 ||
			(!xmuxClient.UnreusableAt.IsZero() && time.Now().After(xmuxClient.UnreusableAt)) {

			m.logger.DebugContext(ctx, "xhttp: removing expired xmuxClient",
				"closed", xmuxClient.IsClosed(),
				"openUsage", xmuxClient.OpenUsage.Load(),
				"leftUsage", xmuxClient.leftUsage,
				"leftRequests", xmuxClient.LeftRequests.Load(),
				"unreusableAt", xmuxClient.UnreusableAt)

			// Remove from slice
			m.xmuxClients = append(m.xmuxClients[:i], m.xmuxClients[i+1:]...)
		} else {
			i++
		}
	}

	// Create new client if pool is empty
	if len(m.xmuxClients) == 0 {
		m.logger.DebugContext(ctx, "xhttp: creating new xmuxClient (pool empty)")
		return m.newXmuxClient()
	}

	// Create new client if under connection limit
	if m.connections > 0 && len(m.xmuxClients) < int(m.connections) {
		m.logger.DebugContext(ctx, "xhttp: creating new xmuxClient (under max connections)", "poolSize", len(m.xmuxClients))
		return m.newXmuxClient()
	}

	// Filter clients that haven't hit concurrency limit
	availableClients := make([]*XmuxClient, 0)
	if m.concurrency > 0 {
		for _, xmuxClient := range m.xmuxClients {
			if xmuxClient.OpenUsage.Load() < m.concurrency {
				availableClients = append(availableClients, xmuxClient)
			}
		}
	} else {
		availableClients = m.xmuxClients
	}

	// Create new client if all existing clients hit concurrency limit
	if len(availableClients) == 0 {
		m.logger.DebugContext(ctx, "xhttp: creating new xmuxClient (concurrency limit hit)", "poolSize", len(m.xmuxClients))
		return m.newXmuxClient()
	}

	// Select random client from available clients
	i, _ := rand.Int(rand.Reader, big.NewInt(int64(len(availableClients))))
	xmuxClient := availableClients[i.Int64()]

	// Decrement reuse counter
	if xmuxClient.leftUsage > 0 {
		xmuxClient.leftUsage -= 1
	}

	return xmuxClient
}

// Close closes all connections in the pool
func (m *XmuxManager) Close() {
	for _, client := range m.xmuxClients {
		client.Close()
	}
	m.xmuxClients = nil
}
