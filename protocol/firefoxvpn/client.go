package firefoxvpn

import (
	"bytes"
	"context"
	stdTLS "crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/ntp"
)

const (
	defaultFxABaseURL      = "https://api.accounts.firefox.com/v1"
	defaultGuardianBaseURL = "https://vpn.mozilla.org"
)

// ControlPlaneEndpoints holds the base URLs for Firefox Accounts and Guardian.
type ControlPlaneEndpoints struct {
	fxaBaseURL      string
	guardianBaseURL string
}

// NewControlPlaneEndpoints creates a ControlPlaneEndpoints from the given base URLs.
func NewControlPlaneEndpoints(fxaBaseURL, guardianBaseURL string) ControlPlaneEndpoints {
	return ControlPlaneEndpoints{fxaBaseURL: fxaBaseURL, guardianBaseURL: guardianBaseURL}.normalize()
}

// TestNewControlPlaneClientOverride is a test-only hook. When non-nil,
// NewControlPlaneClient delegates to it instead of using production endpoints.
var TestNewControlPlaneClientOverride func(ctx context.Context, apiDetour string) (*ControlPlaneClient, error)

type ControlPlaneClient struct {
	httpClient *http.Client
	endpoints  ControlPlaneEndpoints
	now        func() time.Time
	nonce      io.Reader
}

func NewControlPlaneHTTPClient(ctx context.Context, apiDetour string) (*http.Client, error) {
	var dialContext func(context.Context, string, string) (net.Conn, error)
	if apiDetour == "" {
		fallbackDialer := &net.Dialer{}
		dialContext = fallbackDialer.DialContext
	} else {
		outboundDialer, err := dialer.NewWithOptions(dialer.Options{
			Context:        ctx,
			Options:        option.DialerOptions{Detour: apiDetour},
			RemoteIsDomain: true,
		})
		if err != nil {
			return nil, fmt.Errorf("create control-plane dialer: %w", err)
		}
		dialContext = func(ctx context.Context, network string, addr string) (net.Conn, error) {
			return outboundDialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
		}
	}

	transport := &http.Transport{
		ForceAttemptHTTP2: true,
		TLSClientConfig: &stdTLS.Config{
			Time: ntp.TimeFuncFromContext(ctx),
		},
		DialContext: dialContext,
	}

	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}, nil
}

func NewControlPlaneClient(ctx context.Context, apiDetour string) (*ControlPlaneClient, error) {
	if TestNewControlPlaneClientOverride != nil {
		return TestNewControlPlaneClientOverride(ctx, apiDetour)
	}
	httpClient, err := NewControlPlaneHTTPClient(ctx, apiDetour)
	if err != nil {
		return nil, err
	}
	return newControlPlaneClient(httpClient, defaultControlPlaneEndpoints()), nil
}

// NewControlPlaneClientWithEndpoints creates a ControlPlaneClient with explicit endpoints,
// bypassing production defaults. Used by the test override hook.
func NewControlPlaneClientWithEndpoints(ctx context.Context, apiDetour string, endpoints ControlPlaneEndpoints) (*ControlPlaneClient, error) {
	httpClient, err := NewControlPlaneHTTPClient(ctx, apiDetour)
	if err != nil {
		return nil, err
	}
	return newControlPlaneClient(httpClient, endpoints), nil
}

func newControlPlaneClient(httpClient *http.Client, endpoints ControlPlaneEndpoints) *ControlPlaneClient {
	return &ControlPlaneClient{
		httpClient: httpClient,
		endpoints:  endpoints.normalize(),
		now:        time.Now,
		nonce:      randReader{},
	}
}

func defaultControlPlaneEndpoints() ControlPlaneEndpoints {
	return ControlPlaneEndpoints{
		fxaBaseURL:      defaultFxABaseURL,
		guardianBaseURL: defaultGuardianBaseURL,
	}
}

func (e ControlPlaneEndpoints) normalize() ControlPlaneEndpoints {
	e.fxaBaseURL = strings.TrimRight(e.fxaBaseURL, "/")
	e.guardianBaseURL = strings.TrimRight(e.guardianBaseURL, "/")
	return e
}

func newJSONRequest(ctx context.Context, method string, rawURL string, body []byte) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	return request, nil
}

func readResponseBody(response *http.Response) ([]byte, error) {
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	return body, nil
}
