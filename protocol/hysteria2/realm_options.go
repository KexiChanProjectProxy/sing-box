package hysteria2

import (
	"time"

	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-quic/hysteria2/realm"
	E "github.com/sagernet/sing/common/exceptions"
)

func applyHysteria2RealmExtras(dst *realm.Options, src option.Hysteria2Realm) error {
	prefer, err := realm.ParsePreferIPVersion(src.PreferIPVersion)
	if err != nil {
		return E.Cause(err, "realm.prefer_ip_version")
	}
	ports, err := realm.ParseListenPorts(src.ListenPorts)
	if err != nil {
		return E.Cause(err, "realm.listen_ports")
	}
	if _, err = realm.NormalizeIPv6LookupURL(src.IPv6API); err != nil {
		return E.Cause(err, "realm.ipv6_api")
	}
	dst.PreferIPVersion = prefer
	dst.FallbackTimeout = time.Duration(src.FallbackTimeout)
	dst.IPv6API = src.IPv6API
	dst.ListenPorts = ports
	return nil
}
