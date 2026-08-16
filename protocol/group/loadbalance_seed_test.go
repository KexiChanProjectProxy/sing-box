package group

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	M "github.com/sagernet/sing/common/metadata"
)

type seedStub struct {
	tag string
}

func (s *seedStub) Type() string           { return "direct" }
func (s *seedStub) Tag() string            { return s.tag }
func (s *seedStub) Network() []string      { return []string{"tcp"} }
func (s *seedStub) Dependencies() []string { return nil }
func (s *seedStub) DialContext(context.Context, string, M.Socksaddr) (net.Conn, error) {
	return nil, errors.New("unused")
}
func (s *seedStub) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	return nil, errors.New("unused")
}

func TestSeedInitialSnapshotAllowsSelectBeforeHealth(t *testing.T) {
	t.Parallel()
	lb := &LoadBalance{
		primaryTags: []string{"a", "b"},
		primaryOutbounds: map[string]adapter.Outbound{
			"a": &seedStub{tag: "a"},
			"b": &seedStub{tag: "b"},
		},
		emptyPoolAction: "error",
		strategy:        "random",
	}
	lb.seedInitialSnapshot()
	got, err := lb.selectCandidate(adapter.InboundContext{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Tag != "a" && got.Tag != "b" {
		t.Fatalf("seeded select got %q", got.Tag)
	}
}

func TestEmptySnapshotStillHonorsErrorAction(t *testing.T) {
	t.Parallel()
	lb := &LoadBalance{
		primaryTags:      []string{"a"},
		primaryOutbounds: map[string]adapter.Outbound{"a": &seedStub{tag: "a"}},
		emptyPoolAction:  "error",
	}
	lb.snapshot.Store(&CandidateSnapshot{})
	_, err := lb.selectCandidate(adapter.InboundContext{})
	if err == nil || !strings.Contains(err.Error(), "empty candidate pool") {
		t.Fatalf("expected empty pool error, got %v", err)
	}
}
