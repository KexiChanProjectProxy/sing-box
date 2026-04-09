package urltest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestProbeKeyCanonicalizesDefaultURL(t *testing.T) {
	implicit := NewProbeConfig("urltest", "", 5*time.Second, "interval=1m", "tolerance=50")
	explicit := NewProbeConfig("urltest", DefaultLink, 5*time.Second, "interval=1m", "tolerance=50")

	assert.Equal(t, implicit.Signature(), explicit.Signature())
	assert.Equal(t, ProbeKey("leaf-a", implicit), ProbeKey("leaf-a", explicit))
}

func TestProbeKeyIsolationByConfig(t *testing.T) {
	base := NewProbeConfig("urltest", DefaultLink, 5*time.Second, "interval=1m", "tolerance=50")
	otherURL := NewProbeConfig("urltest", "https://example.com/generate_204", 5*time.Second, "interval=1m", "tolerance=50")
	otherTimeout := NewProbeConfig("urltest", DefaultLink, 10*time.Second, "interval=1m", "tolerance=50")
	otherPolicy := NewProbeConfig("urltest", DefaultLink, 5*time.Second, "interval=3m", "tolerance=50")
	otherKind := NewProbeConfig("loadbalance", DefaultLink, 5*time.Second, "interval=1m", "tolerance=50")

	assert.NotEqual(t, ProbeKey("leaf-a", base), ProbeKey("leaf-a", otherURL))
	assert.NotEqual(t, ProbeKey("leaf-a", base), ProbeKey("leaf-a", otherTimeout))
	assert.NotEqual(t, ProbeKey("leaf-a", base), ProbeKey("leaf-a", otherPolicy))
	assert.NotEqual(t, ProbeKey("leaf-a", base), ProbeKey("leaf-a", otherKind))
}
