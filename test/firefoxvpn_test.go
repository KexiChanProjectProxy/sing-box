package main

import (
	"context"
	"testing"

	box "github.com/sagernet/sing-box"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/firefoxvpn"
	"github.com/stretchr/testify/require"
)

func TestFirefoxVPNRegistersAndStartsWithDualDependencies(t *testing.T) {
	t.Parallel()

	startInstance(t, firefoxVPNOptions(
		firefoxVPNDirectOutbound("api-upstream"),
		firefoxVPNDirectOutbound("data-upstream"),
		firefoxVPNOutbound("firefox-vpn", "data-upstream", "api-upstream"),
	))
}

func TestFirefoxVPNMissingApiDetourFails(t *testing.T) {
	t.Parallel()

	err := startInstanceExpectError(t, firefoxVPNOptions(
		firefoxVPNDirectOutbound("data-upstream"),
		firefoxVPNOutbound("firefox-vpn", "data-upstream", "missing-api"),
	))
	require.ErrorContains(t, err, "dependency[missing-api] not found for outbound[firefox-vpn]")
}

func TestFirefoxVPNMissingDataDetourFails(t *testing.T) {
	t.Parallel()

	err := startInstanceExpectError(t, firefoxVPNOptions(
		firefoxVPNDirectOutbound("api-upstream"),
		firefoxVPNOutbound("firefox-vpn", "missing-data", "api-upstream"),
	))
	require.ErrorContains(t, err, "dependency[missing-data] not found for outbound[firefox-vpn]")
}

func TestFirefoxVPNCircularDependencyFails(t *testing.T) {
	t.Parallel()

	err := startInstanceExpectError(t, firefoxVPNOptions(
		option.Outbound{
			Type: C.TypeSelector,
			Tag:  "api-selector",
			Options: &option.SelectorOutboundOptions{
				Outbounds: []string{"firefox-vpn"},
			},
		},
		firefoxVPNOutbound("firefox-vpn", "", "api-selector"),
	))
	require.ErrorContains(t, err, "circular outbound dependency: api-selector -> firefox-vpn -> api-selector")
}

func TestFirefoxVPNEqualDetoursDedupDependencies(t *testing.T) {
	t.Parallel()

	rawOutbound, err := firefoxvpn.NewOutbound(context.Background(), nil, log.NewNOPFactory().Logger(), "firefox-vpn", option.FirefoxVPNOutboundOptions{
		DialerOptions: option.DialerOptions{Detour: "shared-upstream"},
		APIDetour:     "shared-upstream",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"shared-upstream"}, rawOutbound.Dependencies())

	startInstance(t, firefoxVPNOptions(
		firefoxVPNDirectOutbound("shared-upstream"),
		firefoxVPNOutbound("firefox-vpn", "shared-upstream", "shared-upstream"),
	))
}

func firefoxVPNOptions(outbounds ...option.Outbound) option.Options {
	return option.Options{Outbounds: outbounds}
}

func firefoxVPNDirectOutbound(tag string) option.Outbound {
	return option.Outbound{
		Type:    C.TypeDirect,
		Tag:     tag,
		Options: &option.DirectOutboundOptions{},
	}
}

func firefoxVPNOutbound(tag string, detour string, apiDetour string) option.Outbound {
	return option.Outbound{
		Type: C.TypeFirefoxVPN,
		Tag:  tag,
		Options: &option.FirefoxVPNOutboundOptions{
			DialerOptions: option.DialerOptions{Detour: detour},
			ServerOptions: option.ServerOptions{
				Server:     "vpn.example.test",
				ServerPort: 443,
			},
			APIDetour: apiDetour,
			Email:     "user@example.com",
			Password:  "correct horse battery staple",
		},
	}
}

func startInstanceExpectError(t *testing.T, options option.Options) error {
	t.Helper()

	options.Log = &option.LogOptions{Level: "warning"}
	ctx, cancel := context.WithCancel(globalCtx)
	t.Cleanup(cancel)

	instance, err := box.New(box.Options{
		Context: ctx,
		Options: options,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = instance.Close()
	})

	return instance.Start()
}
