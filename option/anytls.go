package option

import (
	"net/url"

	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
)

type AnyTLSInboundOptions struct {
	ListenOptions
	InboundTLSOptionsContainer
	Users         []AnyTLSUser               `json:"users,omitempty"`
	PaddingScheme badoption.Listable[string] `json:"padding_scheme,omitempty"`
	Masquerade    *AnyTLSMasquerade          `json:"masquerade,omitempty"`
}

type AnyTLSUser struct {
	Name     string `json:"name,omitempty"`
	Password string `json:"password,omitempty"`
}

type _AnyTLSMasquerade struct {
	Type            string                   `json:"type,omitempty"`
	FileOptions     AnyTLSMasqueradeFile     `json:"-"`
	ProxyOptions    AnyTLSMasqueradeProxy    `json:"-"`
	StringOptions   AnyTLSMasqueradeString   `json:"-"`
	RedirectOptions AnyTLSMasqueradeRedirect `json:"-"`
}

type AnyTLSMasquerade _AnyTLSMasquerade

func (m AnyTLSMasquerade) MarshalJSON() ([]byte, error) {
	var v any
	switch m.Type {
	case C.AnyTLSMasqueradeTypeFile:
		v = m.FileOptions
	case C.AnyTLSMasqueradeTypeProxy:
		v = m.ProxyOptions
	case C.AnyTLSMasqueradeTypeString:
		v = m.StringOptions
	case C.AnyTLSMasqueradeTypeRedirect:
		v = m.RedirectOptions
	default:
		return nil, E.New("unknown masquerade type: ", m.Type)
	}
	return badjson.MarshallObjects((_AnyTLSMasquerade)(m), v)
}

func (m *AnyTLSMasquerade) UnmarshalJSON(bytes []byte) error {
	var urlString string
	err := json.Unmarshal(bytes, &urlString)
	if err == nil {
		masqueradeURL, err := url.Parse(urlString)
		if err != nil {
			return E.Cause(err, "invalid masquerade URL")
		}
		switch masqueradeURL.Scheme {
		case "file":
			m.Type = C.AnyTLSMasqueradeTypeFile
			m.FileOptions.Directory = masqueradeURL.Path
		case "http", "https":
			m.Type = C.AnyTLSMasqueradeTypeProxy
			m.ProxyOptions.URL = urlString
		default:
			return E.New("unknown masquerade URL scheme: ", masqueradeURL.Scheme)
		}
		return nil
	}
	err = json.Unmarshal(bytes, (*_AnyTLSMasquerade)(m))
	if err != nil {
		return err
	}
	var v any
	switch m.Type {
	case C.AnyTLSMasqueradeTypeFile:
		v = &m.FileOptions
	case C.AnyTLSMasqueradeTypeProxy:
		v = &m.ProxyOptions
	case C.AnyTLSMasqueradeTypeString:
		v = &m.StringOptions
	case C.AnyTLSMasqueradeTypeRedirect:
		v = &m.RedirectOptions
	default:
		return E.New("unknown masquerade type: ", m.Type)
	}
	return badjson.UnmarshallExcluded(bytes, (*_AnyTLSMasquerade)(m), v)
}

type AnyTLSMasqueradeFile struct {
	Directory string `json:"directory"`
}

type AnyTLSMasqueradeProxy struct {
	URL         string `json:"url"`
	RewriteHost bool   `json:"rewrite_host,omitempty"`
}

type AnyTLSMasqueradeString struct {
	StatusCode int                  `json:"status_code,omitempty"`
	Headers    badoption.HTTPHeader `json:"headers,omitempty"`
	Content    string               `json:"content"`
}

type AnyTLSMasqueradeRedirect struct {
	URL              string               `json:"url"`
	StatusCode       int                  `json:"status_code,omitempty"`
	Headers          badoption.HTTPHeader `json:"headers,omitempty"`
	AppendRequestURI bool                 `json:"append_request_uri,omitempty"`
}

type AnyTLSOutboundOptions struct {
	DialerOptions
	ServerOptions
	OutboundTLSOptionsContainer
	Password                     string             `json:"password,omitempty"`
	IdleSessionCheckInterval     badoption.Duration `json:"idle_session_check_interval,omitempty"`
	IdleSessionTimeout           badoption.Duration `json:"idle_session_timeout,omitempty"`
	MinIdleSession               int                `json:"min_idle_session,omitempty"`
	MinIdleSessionForAge         int                `json:"min_idle_session_for_age,omitempty"`
	EnsureIdleSession            int                `json:"ensure_idle_session,omitempty"`
	EnsureIdleSessionCreateRate  int                `json:"ensure_idle_session_create_rate,omitempty"`
	Heartbeat                    badoption.Duration `json:"heartbeat,omitempty"`
	MaxConnectionLifetime        badoption.Duration `json:"max_connection_lifetime,omitempty"`
	ConnectionLifetimeJitter     badoption.Duration `json:"connection_lifetime_jitter,omitempty"`
}
