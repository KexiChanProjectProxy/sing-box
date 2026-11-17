package group

import (
	"context"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/service"

	"github.com/stretchr/testify/require"
)

// A group created after the DNS servers started looks the members up while it
// starts, which must not deadlock on the manager.
func TestGroupCreatedAfterStart(t *testing.T) {
	logger := log.NewNOPFactory().Logger()
	registry := dns.NewTransportRegistry()
	dns.RegisterTransport[option.RemoteDNSServerOptions](registry, C.DNSTypeUDP, func(ctx context.Context, logger log.StructuredLogger, tag string, options option.RemoteDNSServerOptions) (adapter.DNSTransport, error) {
		return &stubLifecycleMember{stubMember: stubMember{tag: tag, memberType: C.DNSTypeUDP}}, nil
	})
	RegisterTransport(registry)
	manager := dns.NewTransportManager(logger, registry, nil, "")
	manager.Initialize(func() (adapter.DNSTransport, error) {
		return &stubLifecycleMember{stubMember: stubMember{tag: "default", memberType: C.DNSTypeUDP}}, nil
	})
	for _, stage := range adapter.ListStartStages {
		require.NoError(t, manager.Start(stage))
	}
	ctx := service.ContextWith[adapter.DNSTransportManager](context.Background(), manager)
	require.NoError(t, manager.Create(ctx, logger, "a", C.DNSTypeUDP, &option.RemoteDNSServerOptions{}))

	done := make(chan error, 1)
	go func() {
		done <- manager.Create(ctx, logger, "group", C.DNSTypeGroup, &option.GroupDNSServerOptions{Servers: []string{"a"}})
	}()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("creating a group after start deadlocked")
	}
	transport, loaded := manager.Transport("group")
	require.True(t, loaded)
	require.Len(t, transport.(*Transport).servers, 1)
}

type stubLifecycleMember struct {
	stubMember
}

func (m *stubLifecycleMember) Start(adapter.StartStage) error { return nil }
func (m *stubLifecycleMember) Close() error                   { return nil }
func (m *stubLifecycleMember) Dependencies() []string         { return nil }
func (m *stubLifecycleMember) Reset()                         {}
