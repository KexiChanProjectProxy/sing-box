# Learnings

## Dependency Delta Analysis (Task 2)

### Key Findings

1. **sing/sing-tun/sing-quic**: Upstream v1.13.11 versions (v0.8.9, v0.8.9, v0.6.1) were adopted over fork's older versions (v0.8.2, v0.8.2, v0.6.0).

2. **tailscale**: Upstream mod.7 was adopted over fork's mod.6 with embedded pseudo-version timestamp.

3. **sing-anytls replace directive**: CRITICAL - Must preserve `replace github.com/anytls/sing-anytls => github.com/KexiChanProjectProxy/sing-anytls v0.0.13`. This redirects to a fork proxy and is NOT the same as the direct require.

4. **Fork-only dependencies**: gin, gorilla/websocket, metacubex/tfo-go, metacubex/utls, cespare/xxhash/v2 are NOT in upstream but are required for fork features.

5. **quic-go appears twice**: `github.com/sagernet/quic-go` (direct, fork-modified) and `github.com/quic-go/quic-go` (indirect, standard) are DIFFERENT packages.

6. **cronet-go timestamp migration**: All 35 cronet-go packages (direct + arch-specific indirect) migrated from `20260303101018` to `20260413093659`.

7. **nftables format change**: Version changed from `-beta.4` to `-mod.2` between fork and upstream.

8. **openai-go upgrade**: Fork's v3.24.0 → upstream's v3.26.0 (minor version bump).

### Pattern: Fork-Derived Upstream Merge
When a fork merges back to upstream, dependency versions typically:
- Adopt upstream's newer versions for shared dependencies
- Preserve fork-only dependencies (gin, gorilla/websocket, etc.)
- Preserve replace directives pointing to fork proxies
- Update timestamp-based pseudo-versions to newer upstream timestamps

## Option Schema Analysis (Task 3)

### Key Findings

1. **V2RayXHTTP* types are fork-only**: Upstream v1.13.11 does NOT provide V2RayXHTTPOptions, V2RayXHTTPXmuxConfig, or V2RayXHTTPRangeConfig. These are fork-owned schema extensions.

2. **Upstream schema changes broke transport/v2rayxhttp/consumer code**:
   - V2RayHTTPOptions.Host changed from `string` to `badoption.Listable[string]` (upstream adoption)
   - tls.Config.Config() method no longer exists (API change in common/tls)
   - These are consumer-level issues in transport/v2rayxhttp/, not option schema issues

3. **Option schema resolution**: Added V2RayXHTTP* types to option/v2ray_transport.go (fork-owned file). `go build ./option/...` now passes.

4. **No xhttp in V2RayTransportOptions union**: xhttp transport is NOT registered in the generic V2RayTransportOptions switch statement because:
   - xhttp is fork-only with no upstream equivalent
   - It implements adapter.V2RayClientTransport/ServerTransport directly
   - Instantiation happens through fork-specific code paths, not the generic transport switch

5. **Schema ownership model**:
   - option/v2ray_transport.go: Contains both upstream types (V2RayHTTPOptions etc.) and fork types (V2RayXHTTP* etc.)
   - transport/v2rayxhttp/: Fork-only transport implementation requiring fork-owned option types
   - V2RayXHTTPOptions embeds V2RayHTTPOptions to inherit upstream HTTP semantics while extending with xhttp-specific fields

### Pattern: Fork-Only Transport Schema Extension
When upstream lacks a transport type that fork implements:
- Define fork-owned option types in option/ package (fork-owned schema file)
- Implement fork transport in transport/<name>/ (fork-owned implementation)
- Do NOT add to upstream's V2RayTransportOptions union type
- Document as fork-only in comments

## Route Core Reconciliation (Task 4)

### Key Findings

1. **No merge conflicts detected**: The current state represents a clean merge result from the initial merge commit e8e05d124.

2. **Fork-owned route fields preserved and wired**:
   - `asnReader` (adapter.ASNReader): Initialized from `options.ASN.Path` in Start(), closed in Close()
   - `geositeReader` (adapter.GeositeReader): Initialized from `options.Geosite.Path` in Start(), closed in Close()
   - `sniffOverrideDestination` (bool): Set from `options.SniffOverrideDestination`, used in actionSniff()

3. **sniffOverrideDestination wiring**: Uses OR logic with action.OverrideDestination:
   ```go
   if (action.OverrideDestination || r.sniffOverrideDestination) && M.IsDomainName(metadata.Domain)
   ```
   This preserves fork's global sniff override while allowing upstream's per-rule control.

4. **Upstream behavior fully adopted**: route.go grew from 785 to 1027 lines with upstream changes including:
   - Event-based structured logging (log.NewConnectionEvent, log.NewRouterMatchEvent, etc.)
   - metadata.Mark = C.DefaultMetadataMark fallback
   - PreferDomainOverrider interface support
   - RevertOriginDst support
   - Route options Mark field

5. **router.go grew from 229 to 306 lines**: +77 lines for fork extensions (ASN/geosite paths and readers).

6. **Build and tests pass**: `go build ./route/...` and `go test ./route/...` both succeed.

### Pattern: Fork Extension Preservation in Merged Route
When preserving fork extensions in merged route code:
- Fork fields are additive to upstream struct (no upstream field removal)
- Initialization happens in Start() method with graceful error handling
- Close() handling uses E.Append pattern for error accumulation
- Getter methods expose fork fields to consumers (ASNReader(), GeositeReader(), SniffOverrideDestination())
- Fork-specific logic integrates with upstream via OR conditions where appropriate

### No Conflicts with Upstream Patterns
- matchStates bitmask (upstream) vs MatchedRuleSet map (fork historical): Already reconciled in task 1, current code uses bitmask
- Event logging (fork) vs LogElapsed (upstream other areas): No conflict - different subsystems use different patterns

## VLESS Outbound Pool Reconciliation (Task 5)

### Key Findings

1. **Connection pool explicitly removed**: Commit bd8d2078 removed pool support from VLESS, and the removal is properly preserved in the merged codebase.

2. **Pool support NOT reintroduced**: Search for `connection_pool`, `connectionPool`, `ConnectionPool` in protocol/vless/ finds only:
   - integration_test.go lines that test pool rejection
   - NO matches in outbound.go, inbound.go, or other VLESS runtime files

3. **Pool rejection via json.Decoder.DisallowUnknownFields**: The integration tests verify that any `connection_pool` config is rejected at parse time.

4. **Upstream VLESS improvements adopted**: Two non-pool upstream changes were properly merged:
   - Event-driven structured logging (commit 0929f2a66): All `logger.InfoContext` calls replaced with `log.NewConnectionEvent` + `log.WithConnectionEvent`
   - `IsFqdn()` → `IsDomain()` rename (commit d2fa21d07): Allows potentially valid domain names

5. **sing-vmess dependency**: Uses v0.2.8 (latest) which has no pool support - pool removal is upstream in the dependency chain

6. **Build and tests pass**:
   - `go build ./protocol/vless/...` succeeds
   - `go test ./protocol/vless/...` succeeds with 4 tests passing

### Pattern: Fork Feature Removal Preservation
When preserving an explicit feature removal in merged code:
- Verify no reintroduction via grep for related symbols
- Maintain rejection tests to prevent accidental re-addition
- Accept upstream improvements in other areas (logging, naming)
- Dependencies should align with removal intent (sing-vmess v0.2.8 has no pool)

## Fork-Only Transport Reconciliation (Task 6)

### Key Findings

1. **Both fork-only transports compile successfully after adaptation**:
   - `go build ./transport/v2rayxhttp/...` succeeds
   - `go build ./transport/v2raycloudflared/...` succeeds

2. **Two API changes required adaptation**:
   - `tls.Config.Config()` renamed to `tls.Config.STDConfig()` in upstream
   - `V2RayHTTPOptions.Host` type changed from `string` to `badoption.Listable[string]`

3. **v2raycloudflared required no changes**: The single-file transport had no API incompatibilities with upstream.

4. **V2RayXHTTPOptions inherits upstream changes via embedding**: The fork-owned `V2RayXHTTPOptions` embeds `V2RayHTTPOptions`, so it automatically got the `Host` type change. Only the transport consumer code (client.go, server.go) needed adaptation.

### Changes Made

**transport/v2rayxhttp/client.go**:
- Line 70: `c.tlsConfig.Config()` → `c.tlsConfig.STDConfig()`
- `buildRequestURL()`: Handle `Listable[string]` Host:
  - Default to server address when Host list is empty
  - Use first Host value when list has entries

**transport/v2rayxhttp/server.go**:
- Added `"slices"` import
- Host validation: `s.config.Host != ""` → `len(s.config.Host) > 0`
- Host comparison: `request.Host != s.config.Host` → `!slices.Contains(s.config.Host, request.Host)`

### Pattern: Fork Transport Adaptation to Upstream Interface Changes
When adapting fork transports to upstream interface changes:
- Identify type changes in embedded/imported upstream types
- Update consumer code to handle new type semantics (e.g., `string` → `Listable[string]`)
- Use standard library helpers (`slices.Contains()`) for Listable operations
- Preserve fork-specific behavior (xmux, padding, etc.) unchanged

### Pre-Existing Blockers (Not Introduced by This Task)
- quic-go/qpack mismatch: Dependency-level issue from fork merge
- common/tlsfragment handshake failure: Pre-existing TLS implementation issue

## libbox Reconciliation (Task 7)

### Key Findings

1. **Build passes**: `go build ./experimental/libbox/...` succeeds (EXIT_CODE: 0)

2. **Full tag build passes**: Android-main configuration with all tags compiles successfully.

3. **No upstream API changes caused libbox failures**: All interfaces (router, route, option, protocol, transport) are compatible with libbox after merge.

4. **ProcessInfo API correctly adapted**: Uses `AndroidPackageNames` (slice) correctly at `command_types.go:350`.

5. **Fork-owned extensions preserved**:
   - F-Droid integration (`fdroid.go`, `fdroid_mirrors.go`)
   - Legacy IPC protocol (13 files with `libbox_legacy_ipc` tag)
   - Platform-specific code (Darwin, Linux, Windows, Android)
   - Clash API extensions
   - Group/Selector protocol extensions

6. **Pre-existing issue with libbox_legacy_ipc tag**: When building with `libbox_legacy_ipc` tag, duplicate method declarations occur (10 methods declared in both `command_client.go` and legacy IPC files). This is NOT caused by upstream merge - it exists because the legacy IPC files and `command_client.go` both define the same methods, and Go doesn't support build-tag-based method overriding. Production builds (android-main, apple, windows) do NOT use `libbox_legacy_ipc` tag.

### Fork-Owned libbox Extensions
- F-Droid mirrors list (40+ mirrors worldwide)
- Clash mode management via clashapi
- Connection management via clashapi
- Group/selector protocol operations
- URL test functionality
- System proxy management
- Service pause/resume
- Darwin TUN interface setup
- Android process file descriptor
- Windows service implementation
- Platform-specific link flags

### No Changes Required
- All upstream API changes were already compatible
- libbox uses sing-box/option correctly
- No adaptations needed for sing v0.8.9

### Evidence Files
- `.sisyphus/evidence/task-7-libbox.txt`: Build verification and extension inventory
- `.sisyphus/evidence/task-7-libbox-edge.txt`: libbox_legacy_ipc duplicate method issue documentation

(End of file - total 229 lines)
