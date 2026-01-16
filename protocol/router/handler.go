package router

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sagernet/sing-box/adapter"
	M "github.com/sagernet/sing/common/metadata"
)

// handleHTTPRequest processes incoming HTTP requests
// This is the main entry point for all HTTP requests
func (r *Inbound) handleHTTPRequest(c *gin.Context) {
	ctx := c.Request.Context()

	// Log the request
	r.logger.DebugContext(ctx, "received request: ", c.Request.Method, " ", c.Request.URL.Path, " from ", c.Request.RemoteAddr)

	// Evaluate routes in priority order
	for _, route := range r.routes {
		if route.matcher.matches(c.Request) {
			r.logger.InfoContext(ctx, "matched route: ", route.name, " for ", c.Request.URL.Path)

			// Check for WebSocket upgrade
			if isWebSocketUpgrade(c) {
				r.handleWebSocketUpgrade(c, route)
				return
			}

			// For router inbound, all HTTP requests should be hijacked and forwarded
			r.handleHijackedConnection(c, route)
			return
		}
	}

	// No match - handle fallback
	r.logger.DebugContext(ctx, "no route matched for ", c.Request.URL.Path, ", using fallback")
	r.fallback.handle(r, c)
}

// handleWebSocketUpgrade hijacks connection for WebSocket
func (r *Inbound) handleWebSocketUpgrade(c *gin.Context, route *compiledRoute) {
	ctx := c.Request.Context()
	r.logger.InfoContext(ctx, "handling WebSocket upgrade for route: ", route.name)

	// Hijack the connection
	conn, err := r.hijackConnection(c)
	if err != nil {
		r.logger.ErrorContext(ctx, "failed to hijack WebSocket connection: ", err)
		c.AbortWithStatus(500)
		return
	}

	// Create metadata
	metadata := r.createMetadata(c, route)

	// Forward to target inbound
	onClose := r.trackHijackedConn(conn)
	if err := r.forwardToInbound(ctx, conn, route.targetInbound, metadata, onClose); err != nil {
		r.logger.ErrorContext(ctx, "failed to forward WebSocket to ", route.targetInbound, ": ", err)
		conn.Close()
		onClose(nil)
	}
}

// handleHijackedConnection handles raw TCP after HTTP
func (r *Inbound) handleHijackedConnection(c *gin.Context, route *compiledRoute) {
	ctx := c.Request.Context()
	r.logger.DebugContext(ctx, "hijacking connection for route: ", route.name)

	// Hijack the connection
	conn, err := r.hijackConnection(c)
	if err != nil {
		r.logger.ErrorContext(ctx, "failed to hijack connection: ", err)
		c.AbortWithStatus(500)
		return
	}

	// Create metadata
	metadata := r.createMetadata(c, route)

	// Forward to target inbound
	onClose := r.trackHijackedConn(conn)
	if err := r.forwardToInbound(ctx, conn, route.targetInbound, metadata, onClose); err != nil {
		r.logger.ErrorContext(ctx, "failed to forward connection to ", route.targetInbound, ": ", err)
		conn.Close()
		onClose(nil)
	}
}

// createMetadata creates InboundContext from gin.Context
func (r *Inbound) createMetadata(c *gin.Context, route *compiledRoute) adapter.InboundContext {
	// Parse source address
	sourceAddr := M.ParseSocksaddr(c.Request.RemoteAddr)

	// Parse destination - extract from HTTP request
	destinationAddr := M.Socksaddr{}
	if host := c.Request.Host; host != "" {
		if strings.Contains(host, ":") {
			destinationAddr = M.ParseSocksaddr(host)
		} else {
			// Add default port based on scheme
			if c.Request.TLS != nil {
				destinationAddr = M.ParseSocksaddr(host + ":443")
			} else {
				destinationAddr = M.ParseSocksaddr(host + ":80")
			}
		}
	}

	return adapter.InboundContext{
		Inbound:     r.Tag(),
		InboundType: r.Type(),
		Source:      sourceAddr,
		Destination: destinationAddr,
	}
}

// isWebSocketUpgrade checks if the request is a WebSocket upgrade
func isWebSocketUpgrade(c *gin.Context) bool {
	return strings.EqualFold(c.Request.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(c.Request.Header.Get("Connection")), "upgrade")
}
