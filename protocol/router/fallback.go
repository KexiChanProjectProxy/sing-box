package router

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
)

type fallbackHandler struct {
	fallbackType string
	webroot      string
	indexFiles   []string
	statusCode   int
	targetTag    string
}

func newFallbackHandler(options *option.FallbackOptions) (*fallbackHandler, error) {
	if options == nil {
		return &fallbackHandler{
			fallbackType: "reject",
			statusCode:   404,
		}, nil
	}

	handler := &fallbackHandler{
		fallbackType: options.Type,
		webroot:      options.Webroot,
		indexFiles:   options.Index,
		statusCode:   options.StatusCode,
		targetTag:    options.Target,
	}

	// Validate based on type
	switch options.Type {
	case "static":
		if handler.webroot == "" {
			return nil, E.New("webroot is required for static fallback")
		}
		// Verify webroot exists
		if info, err := os.Stat(handler.webroot); err != nil {
			return nil, E.Cause(err, "webroot does not exist: ", handler.webroot)
		} else if !info.IsDir() {
			return nil, E.New("webroot is not a directory: ", handler.webroot)
		}
		// Set default index files if not specified
		if len(handler.indexFiles) == 0 {
			handler.indexFiles = []string{"index.html", "index.htm"}
		}

	case "reject":
		// Set default status code if not specified
		if handler.statusCode == 0 {
			handler.statusCode = 404
		}

	case "drop":
		// No validation needed

	case "inbound":
		if handler.targetTag == "" {
			return nil, E.New("target inbound tag is required for inbound fallback")
		}

	default:
		return nil, E.New("unknown fallback type: ", options.Type)
	}

	return handler, nil
}

func (f *fallbackHandler) handle(r *Inbound, c *gin.Context) {
	ctx := c.Request.Context()

	switch f.fallbackType {
	case "static":
		f.serveStatic(r, c)
	case "drop":
		f.drop(r, c)
	case "reject":
		f.reject(c)
	case "inbound":
		f.forwardToInbound(r, c)
	default:
		r.logger.ErrorContext(ctx, "unknown fallback type: ", f.fallbackType)
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}

func (f *fallbackHandler) serveStatic(r *Inbound, c *gin.Context) {
	ctx := c.Request.Context()

	// Get requested path
	requestPath := c.Request.URL.Path

	// Clean the path to prevent directory traversal
	cleanPath := filepath.Clean(requestPath)
	if strings.Contains(cleanPath, "..") {
		r.logger.WarnContext(ctx, "attempted directory traversal: ", requestPath)
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	// Build full file path
	fullPath := filepath.Join(f.webroot, cleanPath)

	// Check if it's a directory
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			r.logger.DebugContext(ctx, "file not found: ", fullPath)
			c.AbortWithStatus(http.StatusNotFound)
		} else {
			r.logger.ErrorContext(ctx, "error accessing file: ", err)
			c.AbortWithStatus(http.StatusInternalServerError)
		}
		return
	}

	// If it's a directory, try index files
	if info.IsDir() {
		for _, indexFile := range f.indexFiles {
			indexPath := filepath.Join(fullPath, indexFile)
			if _, err := os.Stat(indexPath); err == nil {
				fullPath = indexPath
				info, _ = os.Stat(fullPath)
				break
			}
		}
		// If still a directory after trying index files, return 403
		if info.IsDir() {
			r.logger.DebugContext(ctx, "directory listing not allowed: ", fullPath)
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
	}

	// Serve the file
	r.logger.DebugContext(ctx, "serving static file: ", fullPath)
	c.File(fullPath)
}

func (f *fallbackHandler) drop(r *Inbound, c *gin.Context) {
	ctx := c.Request.Context()
	r.logger.DebugContext(ctx, "dropping connection (fallback)")

	// Hijack and immediately close the connection
	hijacker, ok := c.Writer.(http.Hijacker)
	if ok {
		conn, _, err := hijacker.Hijack()
		if err == nil {
			conn.Close()
			return
		}
	}

	// If hijacking fails, just abort with error
	c.AbortWithStatus(http.StatusServiceUnavailable)
}

func (f *fallbackHandler) reject(c *gin.Context) {
	c.AbortWithStatus(f.statusCode)
}

func (f *fallbackHandler) forwardToInbound(r *Inbound, c *gin.Context) {
	ctx := c.Request.Context()
	r.logger.InfoContext(ctx, "forwarding to fallback inbound: ", f.targetTag)

	// Hijack the connection
	conn, err := r.hijackConnection(c)
	if err != nil {
		r.logger.ErrorContext(ctx, "failed to hijack connection for fallback: ", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Create metadata
	metadata := adapter.InboundContext{
		Inbound:     r.Tag(),
		InboundType: r.Type(),
		Source:      M.ParseSocksaddr(c.Request.RemoteAddr),
	}

	// Forward to target inbound
	onClose := r.trackHijackedConn(conn)
	if err := r.forwardToInbound(ctx, conn, f.targetTag, metadata, onClose); err != nil {
		r.logger.ErrorContext(ctx, "failed to forward to fallback inbound: ", err)
		conn.Close()
		onClose(nil)
	}
}
