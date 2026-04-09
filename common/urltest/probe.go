package urltest

import (
	"strconv"
	"strings"
	"time"
)

const DefaultLink = "https://www.gstatic.com/generate_204"

type ProbeConfig struct {
	Kind    string
	URL     string
	Timeout time.Duration
	Policy  []string
}

func NewProbeConfig(kind string, url string, timeout time.Duration, policy ...string) ProbeConfig {
	if url == "" {
		url = DefaultLink
	}
	return ProbeConfig{
		Kind:    kind,
		URL:     url,
		Timeout: timeout,
		Policy:  append([]string{}, policy...),
	}
}

func (c ProbeConfig) Signature() string {
	parts := make([]string, 0, 3+len(c.Policy))
	parts = append(parts, c.Kind, c.URL, strconv.FormatInt(c.Timeout.Nanoseconds(), 10))
	parts = append(parts, c.Policy...)
	return strings.Join(parts, "\x00")
}

func ProbeKey(leafTag string, config ProbeConfig) string {
	if leafTag == "" {
		return ""
	}
	return leafTag + "\x00" + config.Signature()
}
