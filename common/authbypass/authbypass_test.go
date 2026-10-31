package authbypass

import (
	"net/netip"
	"testing"

	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/stretchr/testify/require"
)

func prefixes(values ...string) badoption.Listable[*badoption.Prefixable] {
	var list badoption.Listable[*badoption.Prefixable]
	for _, value := range values {
		p := badoption.Prefixable(netip.MustParsePrefix(value))
		list = append(list, &p)
	}
	return list
}

func TestFor(t *testing.T) {
	users := []auth.User{{Username: "u", Password: "p"}}
	a := New(users, prefixes("10.0.0.0/8", "2001:db8::1/128"))
	require.Nil(t, a.For(M.ParseSocksaddr("10.1.2.3:1234")))
	require.Nil(t, a.For(M.ParseSocksaddr("[::ffff:10.1.2.3]:1234")))
	require.Nil(t, a.For(M.ParseSocksaddr("[2001:db8::1]:1234")))
	require.NotNil(t, a.For(M.ParseSocksaddr("192.168.1.1:1234")))
	require.NotNil(t, a.For(M.ParseSocksaddr("[2001:db8::2]:1234")))
	require.NotNil(t, a.For(M.Socksaddr{}))

	require.NotNil(t, New(users, nil).For(M.ParseSocksaddr("10.1.2.3:1234")))
	require.Nil(t, New(nil, prefixes("10.0.0.0/8")).For(M.ParseSocksaddr("1.1.1.1:1")))
}
