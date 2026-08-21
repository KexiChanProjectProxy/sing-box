package option

import (
	"reflect"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/schema"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badoption"
)

type _OverrideIPOptions struct {
	Server                 string                `json:"server,omitempty" reference:"dns_server"`
	Timeout                badoption.Duration    `json:"timeout,omitempty"`
	Strategy               DomainStrategy        `json:"strategy"`
	DisableCache           bool                  `json:"disable_cache,omitempty"`
	DisableOptimisticCache bool                  `json:"disable_optimistic_cache,omitempty"`
	RewriteTTL             *uint32               `json:"rewrite_ttl,omitempty"`
	ClientSubnet           *badoption.Prefixable `json:"client_subnet,omitempty"`
}

type OverrideIPOptions _OverrideIPOptions

func (o OverrideIPOptions) MarshalJSON() ([]byte, error) {
	if o.strategyOnly() {
		return json.Marshal(o.Strategy)
	}
	return json.Marshal((_OverrideIPOptions)(o))
}

func (o *OverrideIPOptions) UnmarshalJSON(bytes []byte) error {
	var stringValue string
	err := json.Unmarshal(bytes, &stringValue)
	if err == nil {
		var strategy DomainStrategy
		err = json.Unmarshal(bytes, &strategy)
		if err != nil {
			return err
		}
		if C.DomainStrategy(strategy) == C.DomainStrategyAsIS {
			return E.New("empty override_ip strategy")
		}
		o.Strategy = strategy
		return nil
	}
	err = json.Unmarshal(bytes, (*_OverrideIPOptions)(o))
	if err != nil {
		return err
	}
	if C.DomainStrategy(o.Strategy) == C.DomainStrategyAsIS {
		return E.New("missing override_ip.strategy")
	}
	return nil
}

func (o OverrideIPOptions) DescribeSchema(builder schema.Builder) (*schema.Node, error) {
	return builder.Define("OverrideIP", func() (*schema.Node, error) {
		objectForm := schema.StrictObject()
		err := builder.FlattenStruct(objectForm, reflect.TypeFor[OverrideIPOptions]())
		if err != nil {
			return nil, err
		}
		objectForm.Required = []string{"strategy"}
		return schema.AnyOf(
			schema.StringEnum("prefer_ipv4", "prefer_ipv6", "ipv4_only", "ipv6_only"),
			objectForm,
		), nil
	})
}

func (o OverrideIPOptions) strategyOnly() bool {
	return o.Server == "" &&
		o.Timeout == 0 &&
		!o.DisableCache &&
		!o.DisableOptimisticCache &&
		o.RewriteTTL == nil &&
		o.ClientSubnet == nil
}

func CheckDestinationOverride(preferDomain bool, overrideIP *OverrideIPOptions) error {
	if preferDomain && overrideIP != nil {
		return E.New("`prefer_domain` and `override_ip` are mutually exclusive")
	}
	return nil
}
