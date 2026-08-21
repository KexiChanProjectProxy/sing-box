package clickhouse

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	R "github.com/sagernet/sing-box/route/rule"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/stretchr/testify/require"
)

type stubRule struct {
	name   string
	action adapter.RuleAction
}

func (s stubRule) Match(*adapter.InboundContext) bool { return true }
func (s stubRule) String() string                     { return s.name }
func (s stubRule) Type() string                       { return C.RuleTypeDefault }
func (s stubRule) Action() adapter.RuleAction         { return s.action }
func (s stubRule) Start() error                       { return nil }
func (s stubRule) Close() error                       { return nil }

type stubAction struct {
	name string
}

func (s stubAction) Type() string   { return s.name }
func (s stubAction) String() string { return s.name }

func TestBuildSessionEventAndAppendArgs(t *testing.T) {
	tz := time.FixedZone("CST", 8*3600)
	start := time.Date(2026, 8, 22, 0, 21, 13, 482000000, tz)
	end := start.Add(15435 * time.Millisecond)
	mac, err := net.ParseMAC("00:11:22:33:44:55")
	require.NoError(t, err)
	event := buildSessionEvent("sess-1", "gw-01", sessionSnapshot{
		metadata: adapter.InboundContext{
			Inbound:          "mixed-in",
			InboundType:      C.TypeMixed,
			Network:          N.NetworkTCP,
			Protocol:         C.ProtocolTLS,
			User:             "alice",
			Source:           M.SocksaddrFrom(netip.MustParseAddr("10.10.1.23"), 51832),
			Destination:      M.SocksaddrFrom(netip.MustParseAddr("93.184.216.34"), 443),
			Domain:           "www.example.com",
			SourceMACAddress: mac,
			ProcessInfo:      &adapter.ConnectionOwner{ProcessPath: "/usr/bin/curl"},
		},
		rule:         stubRule{name: "geosite-geolocation-!cn", action: stubAction{name: "route"}},
		outboundTag:  "proxy",
		outboundType: C.TypeVLESS,
		chain:        []string{"proxy", "select"},
		start:        start,
		end:          end,
		upload:       18422,
		download:     283714,
		action:       actionAllow,
		close:        closeFin,
	})
	require.Equal(t, "gw-01", event.Node)
	require.Equal(t, int64(15435), event.DurationMs)
	args := event.appendArgs()
	require.Len(t, args, 25)
	require.Equal(t, "gw-01", args[0])
	require.Equal(t, "sess-1", args[1])
	require.Equal(t, start, args[2])
	require.Equal(t, end, args[3])
	require.Equal(t, int64(15435), args[4])
	require.Equal(t, "allow", args[5])
	require.Equal(t, "tcp", args[6])
	require.Equal(t, "tls", args[7])
	require.Equal(t, "alice", args[8])
	require.Equal(t, "10.10.1.23", args[9])
	require.Equal(t, uint16(51832), args[10])
	require.Equal(t, "00:11:22:33:44:55", args[11])
	require.Equal(t, "www.example.com", args[12])
	require.Equal(t, "93.184.216.34", args[13])
	require.Equal(t, uint16(443), args[14])
	require.Equal(t, "mixed-in", args[15])
	require.Equal(t, "mixed", args[16])
	require.Equal(t, "proxy", args[17])
	require.Equal(t, "vless", args[18])
	require.Equal(t, []string{"proxy", "select"}, args[19])
	require.Equal(t, "geosite-geolocation-!cn", args[20])
	require.Equal(t, int64(18422), args[21])
	require.Equal(t, int64(283714), args[22])
	require.Equal(t, "fin", args[23])
	require.Equal(t, "/usr/bin/curl", args[24])
}

func TestAppendArgsEmptyChainIsEmptySlice(t *testing.T) {
	event := buildSessionEvent("sess-2", "gw-01", sessionSnapshot{
		outboundTag:  "direct",
		outboundType: C.TypeDirect,
		chain:        []string{"direct"},
		action:       actionAllow,
	})
	require.Nil(t, event.Chain)
	args := event.appendArgs()
	require.Equal(t, []string{}, args[19])
}

func TestSkipDNSSessions(t *testing.T) {
	require.True(t, skipSession(adapter.InboundContext{Protocol: C.ProtocolDNS}, nil))
	require.False(t, skipSession(adapter.InboundContext{Protocol: C.ProtocolTLS}, nil))
}

func TestRejectCloseReason(t *testing.T) {
	require.Equal(t, closeReject, rejectCloseReason(nil))
	require.Equal(t, closeReject, rejectCloseReason(stubRule{
		action: &R.RuleActionReject{Method: C.RuleActionRejectMethodDefault},
	}))
	require.Equal(t, closeDrop, rejectCloseReason(stubRule{
		action: &R.RuleActionReject{Method: C.RuleActionRejectMethodDrop},
	}))
}

func TestDestinationUsesResolvedAddress(t *testing.T) {
	event := buildSessionEvent("sess-3", "gw-01", sessionSnapshot{
		metadata: adapter.InboundContext{
			Destination:          M.ParseSocksaddrHostPort("www.example.com", 443),
			DestinationAddresses: []netip.Addr{netip.MustParseAddr("93.184.216.34")},
		},
		action: actionAllow,
	})
	require.Equal(t, "www.example.com", event.Destination.Domain)
	require.Equal(t, "93.184.216.34", event.Destination.IP)
	require.Equal(t, uint16(443), event.Destination.Port)
}

func TestParseServerAndInsertQuery(t *testing.T) {
	server, err := parseServer("clickhouse.example.com")
	require.NoError(t, err)
	require.Equal(t, "clickhouse.example.com:9000", server)

	server, err = parseServer("127.0.0.1:9440")
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:9440", server)

	_, err = parseServer("https://clickhouse.example.com")
	require.Error(t, err)
	_, err = parseServer("")
	require.Error(t, err)

	query, err := buildInsertQuery("logs", "sessions")
	require.NoError(t, err)
	require.Equal(t, "INSERT INTO `logs`.`sessions` ("+insertColumns+")", query)

	query, err = buildInsertQuery("", "sessions")
	require.NoError(t, err)
	require.Equal(t, "INSERT INTO `sessions` ("+insertColumns+")", query)

	_, err = buildInsertQuery("logs", "sessions;drop")
	require.Error(t, err)
}
