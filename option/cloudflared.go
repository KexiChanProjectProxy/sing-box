package option

type CloudflaredOutboundOptions struct {
	DialerOptions
	Hostname           string `json:"hostname"`                      // Cloudflare Access hostname (required), e.g. "jp-lqy-at.zenkexi.com"
	CloudflaredVersion string `json:"cloudflared_version,omitempty"` // Version to use in User-Agent header (default: "2026.2.0")
}
