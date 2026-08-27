package hysteria2

import (
	"testing"
	"time"

	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-quic/hysteria2/realm"
	"github.com/sagernet/sing/common/json/badoption"
	"github.com/stretchr/testify/require"
)

func TestApplyHysteria2RealmExtras(t *testing.T) {
	t.Parallel()
	dst := &realm.Options{}
	err := applyHysteria2RealmExtras(dst, option.Hysteria2Realm{
		PreferIPVersion: "v4",
		FallbackTimeout: badoption.Duration(10 * time.Second),
		IPv6API:         "https://api6.ipify.org",
		ListenPorts:     badoption.Listable[string]{"443", "60000-60001"},
	})
	require.NoError(t, err)
	require.Equal(t, realm.PreferIPVersion4, dst.PreferIPVersion)
	require.Equal(t, 10*time.Second, dst.FallbackTimeout)
	require.Equal(t, "https://api6.ipify.org", dst.IPv6API)
	require.Equal(t, []uint16{443, 60000, 60001}, dst.ListenPorts)
}

func TestApplyHysteria2RealmExtrasDefaultPreferV6(t *testing.T) {
	t.Parallel()
	dst := &realm.Options{}
	err := applyHysteria2RealmExtras(dst, option.Hysteria2Realm{})
	require.NoError(t, err)
	require.Equal(t, realm.PreferIPVersion6, dst.PreferIPVersion)
}
