package route

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common/metadata"
	"github.com/stretchr/testify/require"
)

type mockOutbound struct {
	tag          string
	preferDomain bool
}

func (m *mockOutbound) Type() string                     { return "mock" }
func (m *mockOutbound) Tag() string                      { return m.tag }
func (m *mockOutbound) Network() []string                  { return nil }
func (m *mockOutbound) Dependencies() []string            { return nil }
func (m *mockOutbound) PreferDomain() bool               { return m.preferDomain }
func (m *mockOutbound) DialContext(ctx context.Context, network string, destination metadata.Socksaddr) (net.Conn, error) {
	return nil, nil
}
func (m *mockOutbound) ListenPacket(ctx context.Context, destination metadata.Socksaddr) (net.PacketConn, error) {
	return nil, nil
}

type mockOutboundNoInterface struct {
	tag string
}

func (m *mockOutboundNoInterface) Type() string                     { return "mock-no-interface" }
func (m *mockOutboundNoInterface) Tag() string                      { return m.tag }
func (m *mockOutboundNoInterface) Network() []string                  { return nil }
func (m *mockOutboundNoInterface) Dependencies() []string            { return nil }
func (m *mockOutboundNoInterface) DialContext(ctx context.Context, network string, destination metadata.Socksaddr) (net.Conn, error) {
	return nil, nil
}
func (m *mockOutboundNoInterface) ListenPacket(ctx context.Context, destination metadata.Socksaddr) (net.PacketConn, error) {
	return nil, nil
}

func TestApplyPreferDomain(t *testing.T) {
	t.Parallel()

	destinationWithPort443 := metadata.Socksaddr{
		Fqdn: "original.example.com",
		Port: 443,
	}

	destinationWithIPv4Port80 := metadata.Socksaddr{
		Addr: netip.MustParseAddr("1.2.3.4"),
		Port: 80,
	}

	destinationWithIPv6Port443 := metadata.Socksaddr{
		Addr: netip.MustParseAddr("::1"),
		Port: 443,
	}

	testCases := []struct {
		name            string
		metadata        *adapter.InboundContext
		outbound        adapter.Outbound
		expectDest      metadata.Socksaddr
		expectUnchanged bool
	}{
		{
			name: "HTTP protocol with domain rewrites destination Fqdn",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolHTTP,
				Domain:      "example.com",
				Destination: destinationWithPort443,
			},
			outbound: &mockOutbound{tag: "test", preferDomain: true},
			expectDest: metadata.Socksaddr{
				Fqdn: "example.com",
				Port: 443,
			},
		},
		{
			name: "TLS protocol with domain rewrites destination Fqdn",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolTLS,
				Domain:      "tls.example.com",
				Destination: destinationWithPort443,
			},
			outbound: &mockOutbound{tag: "test", preferDomain: true},
			expectDest: metadata.Socksaddr{
				Fqdn: "tls.example.com",
				Port: 443,
			},
		},
		{
			name: "QUIC protocol with domain rewrites destination Fqdn",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolQUIC,
				Domain:      "quic.example.com",
				Destination: destinationWithPort443,
			},
			outbound: &mockOutbound{tag: "test", preferDomain: true},
			expectDest: metadata.Socksaddr{
				Fqdn: "quic.example.com",
				Port: 443,
			},
		},
		{
			name: "IPv4 literal domain rewrites as IP Addr",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolHTTP,
				Domain:      "192.168.1.1",
				Destination: destinationWithPort443,
			},
			outbound: &mockOutbound{tag: "test", preferDomain: true},
			expectDest: metadata.Socksaddr{
				Addr: netip.MustParseAddr("192.168.1.1"),
				Port: 443,
			},
		},
		{
			name: "IPv6 literal domain rewrites as IP Addr",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolHTTP,
				Domain:      "::ffff:127.0.0.1",
				Destination: destinationWithPort443,
			},
			outbound: &mockOutbound{tag: "test", preferDomain: true},
			expectDest: metadata.Socksaddr{
				Addr: netip.MustParseAddr("::ffff:127.0.0.1"),
				Port: 443,
			},
		},
		{
			name: "Disabled outbound (PreferDomain returns false) does not rewrite",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolHTTP,
				Domain:      "example.com",
				Destination: destinationWithPort443,
			},
			outbound: &mockOutbound{tag: "test", preferDomain: false},
			expectDest: metadata.Socksaddr{
				Fqdn: "original.example.com",
				Port: 443,
			},
			expectUnchanged: true,
		},
		{
			name: "Outbound without OutboundWithPreferDomain does not rewrite",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolHTTP,
				Domain:      "example.com",
				Destination: destinationWithPort443,
			},
			outbound: &mockOutboundNoInterface{tag: "test"},
			expectDest: metadata.Socksaddr{
				Fqdn: "original.example.com",
				Port: 443,
			},
			expectUnchanged: true,
		},
		{
			name: "Empty domain does not rewrite",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolHTTP,
				Domain:      "",
				Destination: destinationWithPort443,
			},
			outbound: &mockOutbound{tag: "test", preferDomain: true},
			expectDest: metadata.Socksaddr{
				Fqdn: "original.example.com",
				Port: 443,
			},
			expectUnchanged: true,
		},
		{
			name: "DNS protocol does not rewrite",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolDNS,
				Domain:      "example.com",
				Destination: destinationWithPort443,
			},
			outbound: &mockOutbound{tag: "test", preferDomain: true},
			expectDest: metadata.Socksaddr{
				Fqdn: "original.example.com",
				Port: 443,
			},
			expectUnchanged: true,
		},
		{
			name: "STUN protocol does not rewrite",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolSTUN,
				Domain:      "example.com",
				Destination: destinationWithPort443,
			},
			outbound: &mockOutbound{tag: "test", preferDomain: true},
			expectDest: metadata.Socksaddr{
				Fqdn: "original.example.com",
				Port: 443,
			},
			expectUnchanged: true,
		},
		{
			name: "BitTorrent protocol does not rewrite",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolBitTorrent,
				Domain:      "example.com",
				Destination: destinationWithPort443,
			},
			outbound: &mockOutbound{tag: "test", preferDomain: true},
			expectDest: metadata.Socksaddr{
				Fqdn: "original.example.com",
				Port: 443,
			},
			expectUnchanged: true,
		},
		{
			name: "DTLS protocol does not rewrite",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolDTLS,
				Domain:      "example.com",
				Destination: destinationWithPort443,
			},
			outbound: &mockOutbound{tag: "test", preferDomain: true},
			expectDest: metadata.Socksaddr{
				Fqdn: "original.example.com",
				Port: 443,
			},
			expectUnchanged: true,
		},
		{
			name: "SSH protocol does not rewrite",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolSSH,
				Domain:      "example.com",
				Destination: destinationWithPort443,
			},
			outbound: &mockOutbound{tag: "test", preferDomain: true},
			expectDest: metadata.Socksaddr{
				Fqdn: "original.example.com",
				Port: 443,
			},
			expectUnchanged: true,
		},
		{
			name: "RDP protocol does not rewrite",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolRDP,
				Domain:      "example.com",
				Destination: destinationWithPort443,
			},
			outbound: &mockOutbound{tag: "test", preferDomain: true},
			expectDest: metadata.Socksaddr{
				Fqdn: "original.example.com",
				Port: 443,
			},
			expectUnchanged: true,
		},
		{
			name: "NTP protocol does not rewrite",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolNTP,
				Domain:      "example.com",
				Destination: destinationWithPort443,
			},
			outbound: &mockOutbound{tag: "test", preferDomain: true},
			expectDest: metadata.Socksaddr{
				Fqdn: "original.example.com",
				Port: 443,
			},
			expectUnchanged: true,
		},
		{
			name: "Invalid host string treated as domain Fqdn",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolHTTP,
				Domain:      "not-a-valid-domain-or-ip",
				Destination: destinationWithPort443,
			},
			outbound: &mockOutbound{tag: "test", preferDomain: true},
			expectDest: metadata.Socksaddr{
				Fqdn: "not-a-valid-domain-or-ip",
				Port: 443,
			},
		},
		{
			name: "Already-equal destination Fqdn is idempotent",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolHTTP,
				Domain:      "example.com",
				Destination: metadata.Socksaddr{
					Fqdn: "example.com",
					Port: 443,
				},
			},
			outbound: &mockOutbound{tag: "test", preferDomain: true},
			expectDest: metadata.Socksaddr{
				Fqdn: "example.com",
				Port: 443,
			},
			expectUnchanged: true,
		},
		{
			name: "Already-equal destination IP is idempotent",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolHTTP,
				Domain:      "1.2.3.4",
				Destination: destinationWithIPv4Port80,
			},
			outbound: &mockOutbound{tag: "test", preferDomain: true},
			expectDest: metadata.Socksaddr{
				Addr: netip.MustParseAddr("1.2.3.4"),
				Port: 80,
			},
			expectUnchanged: true,
		},
		{
			name: "Port is preserved after domain rewrite",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolTLS,
				Domain:      "port-test.example.com",
				Destination: metadata.Socksaddr{
					Fqdn: "original.com",
					Port: 8443,
				},
			},
			outbound: &mockOutbound{tag: "test", preferDomain: true},
			expectDest: metadata.Socksaddr{
				Fqdn: "port-test.example.com",
				Port: 8443,
			},
		},
		{
			name: "Port is preserved after IP rewrite",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolQUIC,
				Domain:      "5.6.7.8",
				Destination: destinationWithIPv6Port443,
			},
			outbound: &mockOutbound{tag: "test", preferDomain: true},
			expectDest: metadata.Socksaddr{
				Addr: netip.MustParseAddr("5.6.7.8"),
				Port: 443,
			},
		},
		// Group outbound tests: selector/urltest with group-level prefer_domain
		{
			name: "Selector group-level prefer_domain true rewrites before delegation",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolTLS,
				Domain:      "selector-prefer.example.com",
				Destination: destinationWithPort443,
			},
			outbound: &mockOutbound{tag: "selector-group-a", preferDomain: true},
			expectDest: metadata.Socksaddr{
				Fqdn: "selector-prefer.example.com",
				Port: 443,
			},
		},
		{
			name: "URLTest group-level prefer_domain true rewrites before delegation",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolHTTP,
				Domain:      "urltest-prefer.example.com",
				Destination: destinationWithPort443,
			},
			outbound: &mockOutbound{tag: "urltest-group-b", preferDomain: true},
			expectDest: metadata.Socksaddr{
				Fqdn: "urltest-prefer.example.com",
				Port: 443,
			},
		},
		{
			name: "Selector group-level prefer_domain false does not rewrite despite child preference",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolHTTP,
				Domain:      "child-prefer.example.com",
				Destination: destinationWithPort443,
			},
			outbound: &mockOutbound{tag: "selector-group-disabled", preferDomain: false},
			expectDest: metadata.Socksaddr{
				Fqdn: "original.example.com",
				Port: 443,
			},
			expectUnchanged: true,
		},
		{
			name: "Child outbound directly selected applies own prefer_domain true",
			metadata: &adapter.InboundContext{
				Protocol:    C.ProtocolQUIC,
				Domain:      "direct-child.example.com",
				Destination: destinationWithPort443,
			},
			outbound: &mockOutbound{tag: "direct-shadowsocks-out", preferDomain: true},
			expectDest: metadata.Socksaddr{
				Fqdn: "direct-child.example.com",
				Port: 443,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			originalDest := tc.metadata.Destination
			ApplyPreferDomain(context.Background(), tc.metadata, tc.outbound)
			if tc.expectUnchanged {
				require.Equal(t, originalDest, tc.metadata.Destination, "destination should not have changed")
			} else {
				require.Equal(t, tc.expectDest, tc.metadata.Destination, "destination mismatch")
			}
		})
	}
}

func TestApplyPreferDomainIdempotentWithGroup(t *testing.T) {
	t.Parallel()

	// Idempotency: calling applyPreferDomain twice with the same group outbound
	// and metadata should result in no additional change on the second call.
	meta := &adapter.InboundContext{
		Protocol:    C.ProtocolTLS,
		Domain:      "idempotent.example.com",
		Destination: metadata.Socksaddr{
			Fqdn: "original.example.com",
			Port: 443,
		},
	}
	outbound := &mockOutbound{tag: "idempotent-selector", preferDomain: true}

	// First call: should rewrite
	ApplyPreferDomain(context.Background(), meta, outbound)
	require.Equal(t, metadata.Socksaddr{
		Fqdn: "idempotent.example.com",
		Port: 443,
	}, meta.Destination, "first call should rewrite destination")

	// Second call: should be a no-op since destination already matches domain
	originalDest := meta.Destination
	ApplyPreferDomain(context.Background(), meta, outbound)
	require.Equal(t, originalDest, meta.Destination, "second call should be idempotent")
}

func TestApplyPreferDomainInheritance(t *testing.T) {
	t.Parallel()

	destinationWithPort443 := metadata.Socksaddr{
		Fqdn: "original.example.com",
		Port: 443,
	}

	testCases := []struct {
		name                 string
		inheritedPrefer      bool
		outboundPreferDomain bool
		protocol            string
		expectRewrite       bool
		expectEffectiveFlag bool
	}{
		{
			name:                "Inherited true + child PreferDomain false causes rewrite",
			inheritedPrefer:     true,
			outboundPreferDomain: false,
			protocol:            C.ProtocolTLS,
			expectRewrite:       true,
			expectEffectiveFlag: true,
		},
		{
			name:                "Inherited false + child PreferDomain false does not rewrite",
			inheritedPrefer:     false,
			outboundPreferDomain: false,
			protocol:            C.ProtocolTLS,
			expectRewrite:       false,
			expectEffectiveFlag: false,
		},
		{
			name:                "Inherited true + child PreferDomain true causes rewrite (idempotent)",
			inheritedPrefer:     true,
			outboundPreferDomain: true,
			protocol:            C.ProtocolHTTP,
			expectRewrite:       true,
			expectEffectiveFlag: true,
		},
		{
			name:                "Inherited false + child PreferDomain true causes rewrite (child's own preference)",
			inheritedPrefer:     false,
			outboundPreferDomain: true,
			protocol:            C.ProtocolQUIC,
			expectRewrite:       true,
			expectEffectiveFlag: true,
		},
		{
			name:                "Inherited true + outbound without OutboundWithPreferDomain does not rewrite (protocol mismatch) but context flag is still true",
			inheritedPrefer:     true,
			outboundPreferDomain: false, // will use mockOutboundNoInterface
			protocol:            C.ProtocolDNS,
			expectRewrite:       false,
			expectEffectiveFlag: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var outbound adapter.Outbound
			if tc.outboundPreferDomain {
				outbound = &mockOutbound{tag: "test", preferDomain: true}
			} else {
				outbound = &mockOutbound{tag: "test", preferDomain: false}
			}

			// If we need an outbound without the interface, override
			if tc.name == "Inherited true + outbound without OutboundWithPreferDomain does not rewrite (protocol mismatch) but context flag is still true" {
				outbound = &mockOutboundNoInterface{tag: "test"}
			}

			meta := &adapter.InboundContext{
				Protocol:    tc.protocol,
				Domain:      "rewrite.example.com",
				Destination: destinationWithPort443,
			}

			originalDest := meta.Destination

			// Create context with inherited prefer_domain value
			ctx := context.Background()
			if tc.inheritedPrefer {
				ctx = adapter.ContextWithPreferDomain(ctx, true)
			}

			// Call ApplyPreferDomain and capture returned context
			returnedCtx := ApplyPreferDomain(ctx, meta, outbound)

			// Verify context propagation
			effectiveFlag := adapter.PreferDomainFromContext(returnedCtx)
			require.Equal(t, tc.expectEffectiveFlag, effectiveFlag, "effective flag mismatch")

			// Verify metadata.Destination
			if tc.expectRewrite {
				require.Equal(t, metadata.Socksaddr{
					Fqdn: "rewrite.example.com",
					Port: 443,
				}, meta.Destination, "destination should be rewritten")
			} else {
				require.Equal(t, originalDest, meta.Destination, "destination should not have changed")
			}
		})
	}
}

// TestApplyPreferDomainContextPropagation verifies that ApplyPreferDomain
// correctly propagates the effective prefer_domain flag through context.
func TestApplyPreferDomainContextPropagation(t *testing.T) {
	t.Parallel()

	t.Run("Effective flag is true when outbound PreferDomain is true", func(t *testing.T) {
		t.Parallel()
		meta := &adapter.InboundContext{
			Protocol:    C.ProtocolTLS,
			Domain:      "example.com",
			Destination: metadata.Socksaddr{Fqdn: "original.com", Port: 443},
		}
		outbound := &mockOutbound{tag: "test", preferDomain: true}

		ctx := ApplyPreferDomain(context.Background(), meta, outbound)

		require.True(t, adapter.PreferDomainFromContext(ctx), "effective flag should be true")
	})

	t.Run("Effective flag is true even when protocol does not match (for nested inheritance)", func(t *testing.T) {
		t.Parallel()
		// DNS protocol does not trigger rewrite, but the effective flag should still be set
		meta := &adapter.InboundContext{
			Protocol:    C.ProtocolDNS,
			Domain:      "example.com",
			Destination: metadata.Socksaddr{Fqdn: "original.com", Port: 443},
		}
		outbound := &mockOutbound{tag: "test", preferDomain: true}

		ctx := ApplyPreferDomain(context.Background(), meta, outbound)

		require.True(t, adapter.PreferDomainFromContext(ctx), "effective flag should be true for nested inheritance")
		// Destination should NOT be rewritten because DNS protocol doesn't match
		require.Equal(t, metadata.Socksaddr{Fqdn: "original.com", Port: 443}, meta.Destination)
	})

	t.Run("Effective flag is false when both inherited and outbound are false", func(t *testing.T) {
		t.Parallel()
		meta := &adapter.InboundContext{
			Protocol:    C.ProtocolHTTP,
			Domain:      "example.com",
			Destination: metadata.Socksaddr{Fqdn: "original.com", Port: 80},
		}
		outbound := &mockOutbound{tag: "test", preferDomain: false}

		ctx := ApplyPreferDomain(context.Background(), meta, outbound)

		require.False(t, adapter.PreferDomainFromContext(ctx), "effective flag should be false")
	})

	t.Run("Effective flag inherits from context when outbound is false", func(t *testing.T) {
		t.Parallel()
		meta := &adapter.InboundContext{
			Protocol:    C.ProtocolHTTP,
			Domain:      "example.com",
			Destination: metadata.Socksaddr{Fqdn: "original.com", Port: 80},
		}
		outbound := &mockOutbound{tag: "test", preferDomain: false}

		// Set inherited=true in context
		ctx := adapter.ContextWithPreferDomain(context.Background(), true)
		ctx = ApplyPreferDomain(ctx, meta, outbound)

		require.True(t, adapter.PreferDomainFromContext(ctx), "effective flag should be inherited true")
	})
}

// TestApplyPreferDomainNestedInheritance verifies that effective prefer_domain
// propagates correctly through nested chain: outer group → inner group → child.
func TestApplyPreferDomainNestedInheritance(t *testing.T) {
	t.Parallel()

	t.Run("Chain outer(true) inner(false) child(false) stays true throughout", func(t *testing.T) {
		t.Parallel()
		// Chain: outer group (preferDomain: true) → inner group (preferDomain: false) → child (preferDomain: false)
		meta := &adapter.InboundContext{
			Protocol:    C.ProtocolTLS,
			Domain:      "nested.example.com",
			Destination: metadata.Socksaddr{Fqdn: "original.com", Port: 443},
		}

		// Step 1: outer group with preferDomain: true
		outerOutbound := &mockOutbound{tag: "outer", preferDomain: true}
		ctx := ApplyPreferDomain(context.Background(), meta, outerOutbound)
		require.True(t, adapter.PreferDomainFromContext(ctx), "effective should be true after outer")
		require.Equal(t, metadata.Socksaddr{Fqdn: "nested.example.com", Port: 443}, meta.Destination, "destination should be rewritten by outer")

		// Step 2: inner group with preferDomain: false (but inherited is true, so effective stays true)
		innerOutbound := &mockOutbound{tag: "inner", preferDomain: false}
		ctx = ApplyPreferDomain(ctx, meta, innerOutbound)
		require.True(t, adapter.PreferDomainFromContext(ctx), "effective should still be true (inherited || inner.false = true)")
		// Idempotent: destination already matches domain
		require.Equal(t, metadata.Socksaddr{Fqdn: "nested.example.com", Port: 443}, meta.Destination, "destination should be unchanged (idempotent)")

		// Step 3: child with preferDomain: false (inherited is still true, so effective stays true)
		childOutbound := &mockOutbound{tag: "child", preferDomain: false}
		ctx = ApplyPreferDomain(ctx, meta, childOutbound)
		require.True(t, adapter.PreferDomainFromContext(ctx), "effective should still be true (inherited || child.false = true)")
		require.Equal(t, metadata.Socksaddr{Fqdn: "nested.example.com", Port: 443}, meta.Destination, "destination should be unchanged (idempotent)")
	})

	t.Run("Chain outer(false) inner(true) child(false) becomes true at inner and stays true", func(t *testing.T) {
		t.Parallel()
		// Chain: outer group (preferDomain: false) → inner group (preferDomain: true) → child (preferDomain: false)
		meta := &adapter.InboundContext{
			Protocol:    C.ProtocolTLS,
			Domain:      "nested.example.com",
			Destination: metadata.Socksaddr{Fqdn: "original.com", Port: 443},
		}

		// Step 1: outer group with preferDomain: false
		outerOutbound := &mockOutbound{tag: "outer", preferDomain: false}
		ctx := ApplyPreferDomain(context.Background(), meta, outerOutbound)
		require.False(t, adapter.PreferDomainFromContext(ctx), "effective should be false after outer")
		require.Equal(t, metadata.Socksaddr{Fqdn: "original.com", Port: 443}, meta.Destination, "destination should not be rewritten by outer")

		// Step 2: inner group with preferDomain: true (inherited is false, so effective becomes true)
		innerOutbound := &mockOutbound{tag: "inner", preferDomain: true}
		ctx = ApplyPreferDomain(ctx, meta, innerOutbound)
		require.True(t, adapter.PreferDomainFromContext(ctx), "effective should be true (inherited false || inner true = true)")
		require.Equal(t, metadata.Socksaddr{Fqdn: "nested.example.com", Port: 443}, meta.Destination, "destination should be rewritten by inner")

		// Step 3: child with preferDomain: false (inherited is true, so effective stays true)
		childOutbound := &mockOutbound{tag: "child", preferDomain: false}
		ctx = ApplyPreferDomain(ctx, meta, childOutbound)
		require.True(t, adapter.PreferDomainFromContext(ctx), "effective should still be true (inherited || child.false = true)")
		require.Equal(t, metadata.Socksaddr{Fqdn: "nested.example.com", Port: 443}, meta.Destination, "destination should be unchanged (idempotent)")
	})

	t.Run("Chain outer(false) inner(false) child(true) becomes true at child only", func(t *testing.T) {
		t.Parallel()
		// Chain: outer group (preferDomain: false) → inner group (preferDomain: false) → child (preferDomain: true)
		meta := &adapter.InboundContext{
			Protocol:    C.ProtocolQUIC,
			Domain:      "nested.example.com",
			Destination: metadata.Socksaddr{Fqdn: "original.com", Port: 443},
		}

		// Step 1: outer group with preferDomain: false
		outerOutbound := &mockOutbound{tag: "outer", preferDomain: false}
		ctx := ApplyPreferDomain(context.Background(), meta, outerOutbound)
		require.False(t, adapter.PreferDomainFromContext(ctx), "effective should be false after outer")
		require.Equal(t, metadata.Socksaddr{Fqdn: "original.com", Port: 443}, meta.Destination, "destination should not be rewritten")

		// Step 2: inner group with preferDomain: false
		innerOutbound := &mockOutbound{tag: "inner", preferDomain: false}
		ctx = ApplyPreferDomain(ctx, meta, innerOutbound)
		require.False(t, adapter.PreferDomainFromContext(ctx), "effective should still be false")
		require.Equal(t, metadata.Socksaddr{Fqdn: "original.com", Port: 443}, meta.Destination, "destination should not be rewritten")

		// Step 3: child with preferDomain: true (inherited is false, so effective becomes true)
		childOutbound := &mockOutbound{tag: "child", preferDomain: true}
		ctx = ApplyPreferDomain(ctx, meta, childOutbound)
		require.True(t, adapter.PreferDomainFromContext(ctx), "effective should be true (inherited false || child true = true)")
		require.Equal(t, metadata.Socksaddr{Fqdn: "nested.example.com", Port: 443}, meta.Destination, "destination should be rewritten by child")
	})
}