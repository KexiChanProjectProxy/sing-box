package v2rayxhttp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/net/http2"
)

var _ adapter.V2RayClientTransport = (*Client)(nil)

type Client struct {
	ctx        context.Context
	dialer     N.Dialer
	serverAddr M.Socksaddr
	config     *option.V2RayXHTTPOptions
	tlsConfig  tls.Config
	xmuxMgr    *XmuxManager
}

func NewClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayXHTTPOptions, tlsConfig tls.Config) (*Client, error) {
	options = *normalizeConfig(&options)

	client := &Client{
		ctx:        ctx,
		dialer:     dialer,
		serverAddr: serverAddr,
		config:     &options,
		tlsConfig:  tlsConfig,
	}

	// Create Xmux manager
	client.xmuxMgr = NewXmuxManager(options.Xmux, nil, func() *XmuxClient {
		return &XmuxClient{
			httpClient: client.createHTTPClient(),
		}
	})

	return client, nil
}

func (c *Client) createHTTPClient() *http.Client {
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return c.dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
			ForceAttemptHTTP2: true,
		},
	}

	// Configure HTTP/2 if TLS is enabled
	if c.tlsConfig != nil {
		tlsConfig, err := c.tlsConfig.Config()
		if err == nil {
			transport := httpClient.Transport.(*http.Transport)
			transport.TLSClientConfig = tlsConfig

			// Setup HTTP/2
			if err := http2.ConfigureTransport(transport); err == nil {
				// HTTP/2 configured successfully
			}

			// Configure keep-alive
			if c.config.Xmux != nil && c.config.Xmux.HKeepAlivePeriod > 0 {
				transport.IdleConnTimeout = time.Duration(c.config.Xmux.HKeepAlivePeriod) * time.Second
			}
		}
	}

	return httpClient
}

func (c *Client) DialContext(ctx context.Context) (net.Conn, error) {
	// Generate session UUID
	sessionID := c.generateSessionID()

	// Build request URL
	requestURL := c.buildRequestURL(sessionID)

	// Get Xmux client from pool
	xmuxClient := c.xmuxMgr.GetXmuxClient(ctx)
	xmuxClient.OpenUsage.Add(1)

	// Create pipes for upload/download
	downloadReader, downloadWriter := io.Pipe()
	uploadReader, uploadWriter := io.Pipe()

	// Create split connection
	conn := &splitConn{
		reader:     downloadReader,
		writer:     uploadWriter,
		remoteAddr: c.serverAddr.TCPAddr(),
		localAddr:  nil,
		onClose: func() {
			xmuxClient.OpenUsage.Add(-1)
		},
	}

	// Start GET request for downloading
	go c.handleDownload(ctx, requestURL, downloadWriter, xmuxClient)

	// Start POST goroutine for uploading (packet-up mode)
	go c.handleUpload(ctx, requestURL, uploadReader, xmuxClient)

	return conn, nil
}

func (c *Client) handleDownload(ctx context.Context, baseURL string, writer *io.PipeWriter, xmuxClient *XmuxClient) {
	defer writer.Close()

	// Add x_padding query parameter
	requestURL := c.addPaddingParameter(baseURL)

	// Create GET request
	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		writer.CloseWithError(E.Cause(err, "failed to create GET request"))
		return
	}

	// Add custom headers
	for key, values := range c.config.Headers.Build() {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Decrement request counter
	xmuxClient.LeftRequests.Add(-1)

	// Send request
	resp, err := xmuxClient.httpClient.Do(req)
	if err != nil {
		writer.CloseWithError(E.Cause(err, "failed to send GET request"))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writer.CloseWithError(E.New("unexpected status code: ", resp.StatusCode))
		return
	}

	// Copy response body to writer
	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		writer.CloseWithError(E.Cause(err, "failed to read response"))
		return
	}
}

func (c *Client) handleUpload(ctx context.Context, baseURL string, reader *io.PipeReader, xmuxClient *XmuxClient) {
	defer reader.Close()

	var seq int64
	var lastWrite time.Time
	minInterval := time.Duration(getNormalizedValue(c.config.ScMinPostsIntervalMs)) * time.Millisecond
	maxChunkSize := int(getNormalizedValue(c.config.ScMaxEachPostBytes))

	for {
		// Read chunk from upload pipe
		chunk := buf.New()
		_, err := chunk.ReadFullFrom(reader, maxChunkSize)
		if err != nil {
			if err != io.EOF {
				reader.CloseWithError(E.Cause(err, "failed to read upload data"))
			}
			break
		}

		// Respect minimum interval between POSTs
		if minInterval > 0 && !lastWrite.IsZero() {
			elapsed := time.Since(lastWrite)
			if elapsed < minInterval {
				time.Sleep(minInterval - elapsed)
			}
		}

		// Build request URL with sequence number
		requestURL := baseURL + "/" + strconv.FormatInt(seq, 10)
		requestURL = c.addPaddingParameter(requestURL)

		seq++
		lastWrite = time.Now()

		// Check if we need to rotate xmux client
		if xmuxClient.LeftRequests.Add(-1) <= 0 ||
			(!xmuxClient.UnreusableAt.IsZero() && time.Now().After(xmuxClient.UnreusableAt)) {
			// Get new client
			xmuxClient = c.xmuxMgr.GetXmuxClient(ctx)
			xmuxClient.OpenUsage.Add(1)
		}

		// Send POST request
		go func(data *buf.Buffer, url string) {
			defer data.Release()

			req, err := http.NewRequestWithContext(ctx, "POST", url, data)
			if err != nil {
				return
			}

			// Add custom headers
			for key, values := range c.config.Headers.Build() {
				for _, value := range values {
					req.Header.Add(key, value)
				}
			}

			req.ContentLength = int64(data.Len())

			resp, err := xmuxClient.httpClient.Do(req)
			if err != nil {
				reader.CloseWithError(E.Cause(err, "failed to send POST request"))
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				reader.CloseWithError(E.New("unexpected POST status: ", resp.StatusCode))
				return
			}
		}(chunk, requestURL)
	}
}

func (c *Client) generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (c *Client) buildRequestURL(sessionID string) string {
	var scheme string
	if c.tlsConfig != nil {
		scheme = "https"
	} else {
		scheme = "http"
	}

	host := c.config.Host
	if host == "" {
		host = c.serverAddr.AddrString()
	}

	// Add port if not default
	port := c.serverAddr.Port
	if (scheme == "https" && port != 443) || (scheme == "http" && port != 80) {
		host = net.JoinHostPort(host, strconv.Itoa(int(port)))
	}

	path := c.config.Path
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	path += sessionID

	u := &url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   path,
	}

	return u.String()
}

func (c *Client) addPaddingParameter(requestURL string) string {
	// Generate random padding
	paddingLen := getNormalizedValue(c.config.XPaddingBytes)
	padding := make([]byte, paddingLen)
	for i := range padding {
		padding[i] = 'X'
	}

	u, _ := url.Parse(requestURL)
	q := u.Query()
	q.Set("x_padding", string(padding))
	u.RawQuery = q.Encode()
	return u.String()
}

func (c *Client) Close() error {
	if c.xmuxMgr != nil {
		c.xmuxMgr.Close()
	}
	return nil
}
