package option

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
)

type Hysteria2InboundOptions struct {
	ListenOptions
	UpMbps                int                `json:"up_mbps,omitempty"`
	DownMbps              int                `json:"down_mbps,omitempty"`
	Obfs                  *Hysteria2Obfs     `json:"obfs,omitempty"`
	Users                 []Hysteria2User    `json:"users,omitempty"`
	IgnoreClientBandwidth bool               `json:"ignore_client_bandwidth,omitempty"`
	InboundTLSOptionsContainer
	Masquerade  *Hysteria2Masquerade    `json:"masquerade,omitempty"`
	BrutalDebug bool                    `json:"brutal_debug,omitempty"`
	Realm       *Hysteria2InboundRealm  `json:"realm,omitempty"`
}

type Hysteria2Realm struct {
	ServerURL       string                     `json:"server_url"`
	Token           string                     `json:"token,omitempty"`
	RealmID         string                     `json:"realm_id"`
	STUNServers     badoption.Listable[string] `json:"stun_servers"`
	HTTPClient      *HTTPClientOptions         `json:"http_client,omitempty"`
	ListenPorts     badoption.Listable[uint16]  `json:"listen_ports,omitempty"`
	PreferIPVersion string                     `json:"prefer_ip_version,omitempty"`
	FallbackTimeout string                     `json:"fallback_timeout,omitempty"`
}

type HTTPClientOptions struct{}

type _Hysteria2Realm struct {
	ServerURL       string                     `json:"server_url"`
	Token           string                     `json:"token,omitempty"`
	RealmID         string                     `json:"realm_id"`
	STUNServers     badoption.Listable[string] `json:"stun_servers"`
	HTTPClient      *HTTPClientOptions         `json:"http_client,omitempty"`
	ListenPorts     any                        `json:"listen_ports,omitempty"`
	PreferIPVersion *string                    `json:"prefer_ip_version,omitempty"`
	FallbackTimeout any                        `json:"fallback_timeout,omitempty"`
}

func (r *Hysteria2Realm) UnmarshalJSON(bytes []byte) error {
	var rawRealm _Hysteria2Realm
	err := json.Unmarshal(bytes, &rawRealm)
	if err != nil {
		return err
	}
	r.ServerURL = rawRealm.ServerURL
	r.Token = rawRealm.Token
	r.RealmID = rawRealm.RealmID
	r.STUNServers = rawRealm.STUNServers
	r.HTTPClient = rawRealm.HTTPClient
	ports, err := parseListenPorts(rawRealm.ListenPorts)
	if err != nil {
		return E.Cause(err, "listen_ports")
	}
	r.ListenPorts = badoption.Listable[uint16](ports)
	if rawRealm.PreferIPVersion != nil {
		if *rawRealm.PreferIPVersion == "" {
			return E.New("invalid prefer_ip_version: empty")
		}
		switch *rawRealm.PreferIPVersion {
		case "prefer_ipv4", "prefer_ipv6", "ipv4_only", "ipv6_only":
			r.PreferIPVersion = *rawRealm.PreferIPVersion
		default:
			return E.New("invalid prefer_ip_version: ", *rawRealm.PreferIPVersion)
		}
	}
	switch ft := rawRealm.FallbackTimeout.(type) {
	case string:
		if ft != "" {
			d, err := time.ParseDuration(ft)
			if err != nil {
				return E.Cause(err, "fallback_timeout")
			}
			if d < 0 {
				return E.New("fallback_timeout cannot be negative")
			}
		}
		r.FallbackTimeout = ft
	case float64:
		if ft < 0 {
			return E.New("fallback_timeout cannot be negative")
		}
		r.FallbackTimeout = strconv.Itoa(int(ft)) + "s"
	case int:
		if ft < 0 {
			return E.New("fallback_timeout cannot be negative")
		}
		r.FallbackTimeout = strconv.Itoa(ft) + "s"
	case nil:
		r.FallbackTimeout = ""
	default:
		return E.New("invalid fallback_timeout type")
	}
	if r.ServerURL == "" {
		return E.New("missing server_url")
	}
	if r.RealmID == "" {
		return E.New("missing realm_id")
	}
	return nil
}

type Hysteria2InboundRealm struct {
	Hysteria2Realm
	STUNDomainResolver *DomainResolveOptions `json:"stun_domain_resolver,omitempty"`
	ListenPorts        badoption.Listable[uint16] `json:"listen_ports,omitempty"`
}

type _Hysteria2InboundRealm struct {
	ServerURL          string                     `json:"server_url"`
	Token              string                     `json:"token,omitempty"`
	RealmID            string                     `json:"realm_id"`
	STUNServers     badoption.Listable[string] `json:"stun_servers,omitempty"`
	HTTPClient         *HTTPClientOptions         `json:"http_client,omitempty"`
	ListenPorts        any                        `json:"listen_ports,omitempty"`
	PreferIPVersion    string                     `json:"prefer_ip_version,omitempty"`
	FallbackTimeout    any                        `json:"fallback_timeout,omitempty"`
	STUNDomainResolver *DomainResolveOptions      `json:"stun_domain_resolver,omitempty"`
}

func (r *Hysteria2InboundRealm) UnmarshalJSON(bytes []byte) error {
	err := r.Hysteria2Realm.UnmarshalJSON(bytes)
	if err != nil {
		return err
	}
	var raw struct {
		ListenPorts        any `json:"listen_ports,omitempty"`
		STUNDomainResolver *DomainResolveOptions `json:"stun_domain_resolver,omitempty"`
	}
	err = json.Unmarshal(bytes, &raw)
	if err != nil {
		return err
	}
	r.STUNDomainResolver = raw.STUNDomainResolver
	ports, err := parseListenPorts(raw.ListenPorts)
	if err != nil {
		return E.Cause(err, "listen_ports")
	}
	r.ListenPorts = badoption.Listable[uint16](ports)
	if len(r.ListenPorts) > 0 {
		serverURL, err := url.Parse(r.ServerURL)
		if err == nil {
			if lport := serverURL.Query().Get("lport"); lport != "" {
				return E.New("listen_ports and URL lport parameter both specified")
			}
		}
	}
	return nil
}

func parseListenPorts(v any) ([]uint16, error) {
	if v == nil {
		return nil, nil
	}

	if ports, ok := v.([]uint16); ok {
		for _, p := range ports {
			if p == 0 {
				return nil, E.New("invalid port 0")
			}
		}
		return ports, nil
	}

	if portsAny, ok := v.([]any); ok {
		ports := make([]uint16, 0, len(portsAny))
		for i, p := range portsAny {
			p64, ok := p.(float64)
			if !ok {
				return nil, E.New("invalid port at index ", i)
			}
			if p64 == 0 {
				return nil, E.New("invalid port 0")
			}
			if p64 < 1 || p64 > 65535 {
				return nil, E.New("invalid port ", int(p64))
			}
			ports = append(ports, uint16(p64))
		}
		return ports, nil
	}

	s, ok := v.(string)
	if !ok {
		return nil, E.New("listen_ports must be array or string")
	}

	s = strings.TrimSpace(s)
	if s == "all" || s == "*" {
		return []uint16{0}, nil
	}

	var result []uint16
	items := strings.Split(s, ",")
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if idx := strings.Index(item, "-"); idx >= 0 {
			startStr := strings.TrimSpace(item[:idx])
			endStr := strings.TrimSpace(item[idx+1:])
			start, err := strconv.ParseUint(startStr, 10, 16)
			if err != nil {
				return nil, E.New("invalid port range start: ", startStr)
			}
			end, err := strconv.ParseUint(endStr, 10, 16)
			if err != nil {
				return nil, E.New("invalid port range end: ", endStr)
			}
			if start == 0 {
				return nil, E.New("invalid port 0")
			}
			if end == 0 || end > 65535 {
				return nil, E.New("invalid port: ", end)
			}
			if start > end {
				return nil, E.New("invalid port range: start > end")
			}
			for p := uint16(start); p <= uint16(end); p++ {
				result = append(result, p)
			}
		} else {
			port, err := strconv.ParseUint(item, 10, 16)
			if err != nil {
				return nil, E.New("invalid port: ", item)
			}
			if port == 0 || port > 65535 {
				return nil, E.New("invalid port: ", port)
			}
			result = append(result, uint16(port))
		}
	}
	return result, nil
}

type Hysteria2Obfs struct {
	Type     string `json:"type,omitempty"`
	Password string `json:"password,omitempty"`
}

type Hysteria2User struct {
	Name     string `json:"name,omitempty"`
	Password string `json:"password,omitempty"`
}

type _Hysteria2Masquerade struct {
	Type          string                    `json:"type,omitempty"`
	FileOptions   Hysteria2MasqueradeFile   `json:"-"`
	ProxyOptions  Hysteria2MasqueradeProxy  `json:"-"`
	StringOptions Hysteria2MasqueradeString `json:"-"`
}

type Hysteria2Masquerade _Hysteria2Masquerade

func (m Hysteria2Masquerade) MarshalJSON() ([]byte, error) {
	var v any
	switch m.Type {
	case C.Hysterai2MasqueradeTypeFile:
		v = m.FileOptions
	case C.Hysterai2MasqueradeTypeProxy:
		v = m.ProxyOptions
	case C.Hysterai2MasqueradeTypeString:
		v = m.StringOptions
	default:
		return nil, E.New("unknown masquerade type: ", m.Type)
	}
	return badjson.MarshallObjects((_Hysteria2Masquerade)(m), v)
}

func (m *Hysteria2Masquerade) UnmarshalJSON(bytes []byte) error {
	var urlString string
	err := json.Unmarshal(bytes, &urlString)
	if err == nil {
		masqueradeURL, err := url.Parse(urlString)
		if err != nil {
			return E.Cause(err, "invalid masquerade URL")
		}
		switch masqueradeURL.Scheme {
		case "file":
			m.Type = C.Hysterai2MasqueradeTypeFile
			m.FileOptions.Directory = masqueradeURL.Path
		case "http", "https":
			m.Type = C.Hysterai2MasqueradeTypeProxy
			m.ProxyOptions.URL = urlString
		default:
			return E.New("unknown masquerade URL scheme: ", masqueradeURL.Scheme)
		}
		return nil
	}
	err = json.Unmarshal(bytes, (*_Hysteria2Masquerade)(m))
	if err != nil {
		return err
	}
	var v any
	switch m.Type {
	case C.Hysterai2MasqueradeTypeFile:
		v = &m.FileOptions
	case C.Hysterai2MasqueradeTypeProxy:
		v = &m.ProxyOptions
	case C.Hysterai2MasqueradeTypeString:
		v = &m.StringOptions
	default:
		return E.New("unknown masquerade type: ", m.Type)
	}
	return badjson.UnmarshallExcluded(bytes, (*_Hysteria2Masquerade)(m), v)
}

type Hysteria2MasqueradeFile struct {
	Directory string `json:"directory"`
}

type Hysteria2MasqueradeProxy struct {
	URL         string `json:"url"`
	RewriteHost bool   `json:"rewrite_host,omitempty"`
}

type Hysteria2MasqueradeString struct {
	StatusCode int                  `json:"status_code,omitempty"`
	Headers    badoption.HTTPHeader `json:"headers,omitempty"`
	Content    string               `json:"content"`
}

type Hysteria2OutboundOptions struct {
	DialerOptions
	ServerOptions
	ServerPorts badoption.Listable[string] `json:"server_ports,omitempty"`
	HopInterval badoption.Duration         `json:"hop_interval,omitempty"`
	UpMbps      int                        `json:"up_mbps,omitempty"`
	DownMbps    int                        `json:"down_mbps,omitempty"`
	Obfs        *Hysteria2Obfs             `json:"obfs,omitempty"`
	Password    string                     `json:"password,omitempty"`
	Network     NetworkList                `json:"network,omitempty"`
	OutboundTLSOptionsContainer
	BrutalDebug bool            `json:"brutal_debug,omitempty"`
	Realm       *Hysteria2Realm `json:"realm,omitempty"`
}

type HysteriaRealmUser struct {
	Name      string `json:"name"`
	Token     string `json:"token"`
	MaxRealms int    `json:"max_realms,omitempty"`
}

type HysteriaRealmServiceOptions struct {
	ListenOptions
	InboundTLSOptionsContainer
	Users []HysteriaRealmUser `json:"users"`
}