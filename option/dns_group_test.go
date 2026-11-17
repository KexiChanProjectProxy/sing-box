package option

import (
	"context"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/service"

	"github.com/stretchr/testify/require"
)

type stubNewDNSTransportOptionsRegistry struct{}

func (stubNewDNSTransportOptionsRegistry) OptionTypes() []string {
	return []string{C.DNSTypeGroup, C.DNSTypeAnyTLS}
}

func (stubNewDNSTransportOptionsRegistry) CreateOptions(transportType string) (any, bool) {
	switch transportType {
	case C.DNSTypeGroup:
		return new(GroupDNSServerOptions), true
	case C.DNSTypeAnyTLS:
		return new(AnyTLSDNSServerOptions), true
	default:
		return nil, false
	}
}

func TestGroupAndAnyTLSDNSServerOptions(t *testing.T) {
	t.Parallel()

	ctx := service.ContextWith[DNSTransportOptionsRegistry](context.Background(), stubNewDNSTransportOptionsRegistry{})
	var options DNSOptions
	err := json.UnmarshalContext(ctx, []byte(`{
		"servers": [
			{"type": "group", "tag": "up", "servers": ["a", "b"], "fallbacks": "c", "mode": "parallel"},
			{"type": "anytls", "tag": "remote", "outbound": "anytls-out"}
		]
	}`), &options)
	require.NoError(t, err)
	require.Len(t, options.Servers, 2)

	group, isGroup := options.Servers[0].Options.(*GroupDNSServerOptions)
	require.True(t, isGroup)
	require.Equal(t, []string{"a", "b"}, []string(group.Servers))
	require.Equal(t, []string{"c"}, []string(group.Fallbacks))
	require.Equal(t, "parallel", group.Mode)

	remote, isAnyTLS := options.Servers[1].Options.(*AnyTLSDNSServerOptions)
	require.True(t, isAnyTLS)
	require.Equal(t, "anytls-out", remote.Outbound)
}
