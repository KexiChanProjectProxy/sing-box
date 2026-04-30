package option

import (
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
)

type _V2RayTransportOptions struct {
	Type               string                  `json:"type"`
	HTTPOptions        V2RayHTTPOptions        `json:"-"`
	WebsocketOptions   V2RayWebsocketOptions   `json:"-"`
	QUICOptions        V2RayQUICOptions        `json:"-"`
	GRPCOptions        V2RayGRPCOptions        `json:"-"`
	HTTPUpgradeOptions V2RayHTTPUpgradeOptions `json:"-"`
	CloudflaredOptions V2RayCloudflaredOptions `json:"-"`
}

type V2RayTransportOptions _V2RayTransportOptions

func (o V2RayTransportOptions) MarshalJSON() ([]byte, error) {
	var v any
	switch o.Type {
	case C.V2RayTransportTypeHTTP:
		v = o.HTTPOptions
	case C.V2RayTransportTypeWebsocket:
		v = o.WebsocketOptions
	case C.V2RayTransportTypeQUIC:
		v = o.QUICOptions
	case C.V2RayTransportTypeGRPC:
		v = o.GRPCOptions
	case C.V2RayTransportTypeHTTPUpgrade:
		v = o.HTTPUpgradeOptions
	case C.V2RayTransportTypeCloudflared:
		v = o.CloudflaredOptions
	case "":
		return nil, E.New("missing transport type")
	default:
		return nil, E.New("unknown transport type: " + o.Type)
	}
	return badjson.MarshallObjects((_V2RayTransportOptions)(o), v)
}

func (o *V2RayTransportOptions) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_V2RayTransportOptions)(o))
	if err != nil {
		return err
	}
	var v any
	switch o.Type {
	case C.V2RayTransportTypeHTTP:
		v = &o.HTTPOptions
	case C.V2RayTransportTypeWebsocket:
		v = &o.WebsocketOptions
	case C.V2RayTransportTypeQUIC:
		v = &o.QUICOptions
	case C.V2RayTransportTypeGRPC:
		v = &o.GRPCOptions
	case C.V2RayTransportTypeHTTPUpgrade:
		v = &o.HTTPUpgradeOptions
	case C.V2RayTransportTypeCloudflared:
		v = &o.CloudflaredOptions
	default:
		return E.New("unknown transport type: " + o.Type)
	}
	err = badjson.UnmarshallExcluded(bytes, (*_V2RayTransportOptions)(o), v)
	if err != nil {
		return err
	}
	return nil
}

type V2RayHTTPOptions struct {
	Host        badoption.Listable[string] `json:"host,omitempty"`
	Path        string                     `json:"path,omitempty"`
	Method      string                     `json:"method,omitempty"`
	Headers     badoption.HTTPHeader       `json:"headers,omitempty"`
	IdleTimeout badoption.Duration         `json:"idle_timeout,omitempty"`
	PingTimeout badoption.Duration         `json:"ping_timeout,omitempty"`
}

type V2RayWebsocketOptions struct {
	Path                string               `json:"path,omitempty"`
	Headers             badoption.HTTPHeader `json:"headers,omitempty"`
	MaxEarlyData        uint32               `json:"max_early_data,omitempty"`
	EarlyDataHeaderName string               `json:"early_data_header_name,omitempty"`
}

type V2RayQUICOptions struct{}

type V2RayGRPCOptions struct {
	ServiceName         string             `json:"service_name,omitempty"`
	IdleTimeout         badoption.Duration `json:"idle_timeout,omitempty"`
	PingTimeout         badoption.Duration `json:"ping_timeout,omitempty"`
	PermitWithoutStream bool               `json:"permit_without_stream,omitempty"`
	ForceLite           bool               `json:"-"` // for test
}

type V2RayHTTPUpgradeOptions struct {
	Host    string               `json:"host,omitempty"`
	Path    string               `json:"path,omitempty"`
	Headers badoption.HTTPHeader `json:"headers,omitempty"`
}

type V2RayCloudflaredOptions struct {
	Hostname           string `json:"hostname"`
	CloudflaredVersion string `json:"cloudflared_version,omitempty"`
}

// V2RayXHTTPRangeConfig is a range config for xhttp parameters (fork-only extension)
type V2RayXHTTPRangeConfig struct {
	From int32 `json:"from"`
	To   int32 `json:"to"`
}

// V2RayXHTTPXmuxConfig is the xmux configuration for xhttp (fork-only extension)
type V2RayXHTTPXmuxConfig struct {
	MaxConcurrency   *V2RayXHTTPRangeConfig `json:"max_concurrency,omitempty"`
	MaxConnections   *V2RayXHTTPRangeConfig `json:"max_connections,omitempty"`
	CMaxReuseTimes   *V2RayXHTTPRangeConfig `json:"c_max_reuse_times,omitempty"`
	HMaxRequestTimes *V2RayXHTTPRangeConfig `json:"h_max_request_times,omitempty"`
	HMaxReusableSecs *V2RayXHTTPRangeConfig `json:"h_max_reusable_secs,omitempty"`
	HKeepAlivePeriod int32                  `json:"h_keep_alive_period,omitempty"`
}

// V2RayXHTTPOptions is the xhttp transport options (fork-only extension)
// It extends V2RayHTTPOptions with xhttp-specific fields
type V2RayXHTTPOptions struct {
	V2RayHTTPOptions
	Mode               string                  `json:"mode,omitempty"`
	XPaddingBytes      *V2RayXHTTPRangeConfig `json:"x_padding_bytes,omitempty"`
	ScMaxEachPostBytes *V2RayXHTTPRangeConfig `json:"sc_max_each_post_bytes,omitempty"`
	ScMinPostsIntervalMs *V2RayXHTTPRangeConfig `json:"sc_min_posts_interval_ms,omitempty"`
	ScMaxBufferedPosts int32                  `json:"sc_max_buffered_posts,omitempty"`
	NoGRPCHeader       bool                   `json:"no_grpc_header,omitempty"`
	Xmux               *V2RayXHTTPXmuxConfig   `json:"xmux,omitempty"`
}
