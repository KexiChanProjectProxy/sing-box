# Decisions


## Merge v1.13.11 Decisions (2026-04-30)

### Event-based Logging Preserved
The fork's event-based logging (`log.NewComponentLifecycleEvent`) was preserved over upstream's `LogElapsed` pattern. Both approaches compile since `LogElapsed` is defined in lifecycle.go.

### ProcessInfo API Migration
sing v0.8.9 changed `AndroidPackageName` (singular string) to `AndroidPackageNames` (slice of strings). Required updates in:
- adapter/log_metadata.go
- route/route.go

### matchStates Architecture
The fork's `MatchedRuleSet` map approach was incompatible with upstream's `ruleMatchStateSet` uint16 bitmask. Had to adopt upstream's matchStates pattern.

### Replace Directive Preserved
`replace github.com/anytls/sing-anytls => github.com/KexiChanProjectProxy/sing-anytls v0.0.13` kept as-is - no verified incompatibility found.
