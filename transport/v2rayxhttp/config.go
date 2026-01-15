package v2rayxhttp

import (
	"math/rand"

	"github.com/sagernet/sing-box/option"
)

// getNormalizedValue returns a random value within the range, or the From value if From == To
func getNormalizedValue(rc *option.V2RayXHTTPRangeConfig) int32 {
	if rc == nil {
		return 0
	}
	if rc.From == rc.To {
		return rc.From
	}
	if rc.From > rc.To {
		return rc.From
	}
	return rc.From + rand.Int31n(rc.To-rc.From+1)
}

// getDefaultXPaddingBytes returns default padding range (100-1000)
func getDefaultXPaddingBytes() *option.V2RayXHTTPRangeConfig {
	return &option.V2RayXHTTPRangeConfig{
		From: 100,
		To:   1000,
	}
}

// getDefaultScMaxEachPostBytes returns default max bytes per POST (1MB)
func getDefaultScMaxEachPostBytes() *option.V2RayXHTTPRangeConfig {
	return &option.V2RayXHTTPRangeConfig{
		From: 1000000,
		To:   1000000,
	}
}

// getDefaultScMinPostsIntervalMs returns default min interval between POSTs (30ms)
func getDefaultScMinPostsIntervalMs() *option.V2RayXHTTPRangeConfig {
	return &option.V2RayXHTTPRangeConfig{
		From: 30,
		To:   30,
	}
}

// getDefaultScMaxBufferedPosts returns default max buffered posts (30)
func getDefaultScMaxBufferedPosts() int32 {
	return 30
}

// normalizeConfig fills in default values for missing configuration options
func normalizeConfig(opts *option.V2RayXHTTPOptions) *option.V2RayXHTTPOptions {
	if opts == nil {
		opts = &option.V2RayXHTTPOptions{}
	}

	// Set default mode if not specified
	if opts.Mode == "" {
		opts.Mode = "auto"
	}

	// Set default path if not specified
	if opts.Path == "" {
		opts.Path = "/"
	}

	// Ensure path ends with /
	if opts.Path[len(opts.Path)-1] != '/' {
		opts.Path += "/"
	}

	// Set default padding if not specified
	if opts.XPaddingBytes == nil {
		opts.XPaddingBytes = getDefaultXPaddingBytes()
	}

	// Set default max bytes per POST if not specified
	if opts.ScMaxEachPostBytes == nil {
		opts.ScMaxEachPostBytes = getDefaultScMaxEachPostBytes()
	}

	// Set default min interval if not specified
	if opts.ScMinPostsIntervalMs == nil {
		opts.ScMinPostsIntervalMs = getDefaultScMinPostsIntervalMs()
	}

	// Set default max buffered posts if not specified
	if opts.ScMaxBufferedPosts == 0 {
		opts.ScMaxBufferedPosts = getDefaultScMaxBufferedPosts()
	}

	// Normalize Xmux config
	if opts.Xmux == nil {
		opts.Xmux = &option.V2RayXHTTPXmuxConfig{}
	}

	// Set Xmux defaults if not specified (0 means unlimited)
	if opts.Xmux.MaxConcurrency == nil {
		opts.Xmux.MaxConcurrency = &option.V2RayXHTTPRangeConfig{From: 0, To: 0}
	}
	if opts.Xmux.MaxConnections == nil {
		opts.Xmux.MaxConnections = &option.V2RayXHTTPRangeConfig{From: 0, To: 0}
	}
	if opts.Xmux.CMaxReuseTimes == nil {
		opts.Xmux.CMaxReuseTimes = &option.V2RayXHTTPRangeConfig{From: 0, To: 0}
	}
	if opts.Xmux.HMaxRequestTimes == nil {
		opts.Xmux.HMaxRequestTimes = &option.V2RayXHTTPRangeConfig{From: 0, To: 0}
	}
	if opts.Xmux.HMaxReusableSecs == nil {
		opts.Xmux.HMaxReusableSecs = &option.V2RayXHTTPRangeConfig{From: 0, To: 0}
	}

	return opts
}
