package option

type CloudflaredOutboundOptions struct {
	DialerOptions
	Hostname string `json:"hostname"` // Cloudflare Access hostname (required), e.g. "jp-lqy-at.zenkexi.com"
}
