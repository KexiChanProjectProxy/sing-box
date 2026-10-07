package rule

import (
	"testing"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"

	"github.com/stretchr/testify/require"
)

// The loadbalance matched_ruleset hash keys read InboundContext.MatchedRuleSetTag,
// which must survive in-place rule matching and its snapshot restores.
func TestMatchedRuleSetTag(t *testing.T) {
	t.Parallel()
	domainSet := func(tag string, domain string) adapter.RuleSet {
		return newLocalRuleSetForTest(tag, headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{domain})
		}))
	}
	ruleSetRule := func(sets ...adapter.RuleSet) *DefaultRule {
		return routeRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: sets})
		})
	}
	t.Run("rule-set match records tag", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		require.True(t, ruleSetRule(domainSet("example", "example.com")).Match(&metadata))
		require.Equal(t, "example", metadata.MatchedRuleSetTag)
	})
	t.Run("first matching rule-set wins", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		rule := ruleSetRule(domainSet("other", "example.org"), domainSet("first", "example.com"), domainSet("second", "www.example.com"))
		require.True(t, rule.Match(&metadata))
		require.Equal(t, "first", metadata.MatchedRuleSetTag)
	})
	t.Run("no match leaves tag empty", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		require.False(t, ruleSetRule(domainSet("other", "example.org")).Match(&metadata))
		require.Empty(t, metadata.MatchedRuleSetTag)
	})
	t.Run("logical rule propagates nested tag", func(t *testing.T) {
		t.Parallel()
		for _, mode := range []string{C.LogicalTypeAnd, C.LogicalTypeOr} {
			metadata := testMetadata("www.example.com")
			logicalRule := &abstractLogicalRule{
				mode: mode,
				rules: []adapter.HeadlessRule{
					ruleSetRule(domainSet("nested", "example.com")),
					routeRuleForTest(func(rule *abstractDefaultRule) {
						addDestinationAddressItem(t, rule, nil, []string{"www.example.com"})
					}),
				},
			}
			require.True(t, logicalRule.Match(&metadata), mode)
			require.Equal(t, "nested", metadata.MatchedRuleSetTag, mode)
		}
	})
}
