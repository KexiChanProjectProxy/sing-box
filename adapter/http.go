package adapter

import (
	"context"
	"github.com/sagernet/sing-box/log"
	"net/http"
	"sync"

	"github.com/sagernet/sing-box/option"
)

type HTTPTransport interface {
	http.RoundTripper
	CloseIdleConnections()
	Reset()
}

type HTTPClientManager interface {
	ResolveTransport(ctx context.Context, logger log.StructuredLogger, options option.HTTPClientOptions) (HTTPTransport, error)
	DefaultTransport() HTTPTransport
	ResetNetwork()
}

type HTTPStartContext struct {
	access     sync.Mutex
	transports []HTTPTransport
}

func NewHTTPStartContext() *HTTPStartContext {
	return &HTTPStartContext{}
}

func (c *HTTPStartContext) Register(transport HTTPTransport) {
	c.access.Lock()
	defer c.access.Unlock()
	c.transports = append(c.transports, transport)
}

func (c *HTTPStartContext) Close() {
	for _, transport := range c.transports {
		transport.CloseIdleConnections()
	}
}
