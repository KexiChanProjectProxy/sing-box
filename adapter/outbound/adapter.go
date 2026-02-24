package outbound

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
)

type Adapter struct {
	outboundType       string
	outboundTag        string
	network            []string
	dependencies       []string
	preferDomainConfig *adapter.PreferDomainConfig
}

func NewAdapter(outboundType string, outboundTag string, network []string, dependencies []string) Adapter {
	return Adapter{
		outboundType: outboundType,
		outboundTag:  outboundTag,
		network:      network,
		dependencies: dependencies,
	}
}

func NewAdapterWithDialerOptions(outboundType string, outboundTag string, network []string, dialOptions option.DialerOptions) Adapter {
	var dependencies []string
	if dialOptions.Detour != "" {
		dependencies = []string{dialOptions.Detour}
	}
	return NewAdapter(outboundType, outboundTag, network, dependencies)
}

func (a *Adapter) Type() string {
	return a.outboundType
}

func (a *Adapter) Tag() string {
	return a.outboundTag
}

func (a *Adapter) Network() []string {
	return a.network
}

func (a *Adapter) Dependencies() []string {
	return a.dependencies
}

// PreferDomainConfig implements adapter.PreferDomainOverrider.
func (a *Adapter) PreferDomainConfig() *adapter.PreferDomainConfig {
	return a.preferDomainConfig
}

// ApplyPreferDomain configures the prefer_domain option from the given options.
func (a *Adapter) ApplyPreferDomain(opts *option.PreferDomainOptions) {
	if opts == nil || !opts.Enabled {
		return
	}
	config := &adapter.PreferDomainConfig{Enabled: true}
	if opts.Mark != nil {
		config.MarkValue = opts.Mark.Value
		config.MarkMask = opts.Mark.Mask
	}
	a.preferDomainConfig = config
}
