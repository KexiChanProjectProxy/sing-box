package route

import (
	"testing"

	"github.com/sagernet/sing-box/option"

	"github.com/stretchr/testify/require"
)

func TestAppendOverrideIPServer(t *testing.T) {
	t.Parallel()
	overrideIP := &option.OverrideIPOptions{Server: "resolver"}
	require.Equal(t, []string{"resolver"}, appendOverrideIPServer(nil, &option.SelectorOutboundOptions{OverrideIP: overrideIP}))
	require.Equal(t, []string{"resolver"}, appendOverrideIPServer(nil, &option.LoadBalanceOutboundOptions{OverrideIP: overrideIP}))
	require.Equal(t, []string{"resolver"}, appendOverrideIPServer(nil, &option.DirectOutboundOptions{DialerOptions: option.DialerOptions{AbstractDialerOptions: option.AbstractDialerOptions{OverrideIP: overrideIP}}}))
	require.Empty(t, appendOverrideIPServer(nil, &option.SelectorOutboundOptions{}))
	require.Empty(t, appendOverrideIPServer(nil, &option.SelectorOutboundOptions{OverrideIP: &option.OverrideIPOptions{}}))
	require.Empty(t, appendOverrideIPServer(nil, (*option.SelectorOutboundOptions)(nil)))
	require.Empty(t, appendOverrideIPServer(nil, nil))
}
