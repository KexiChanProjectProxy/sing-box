
## 2026-07-14 — Task 1: option contract

### badoption.Prefix surface
- Module path is `github.com/sagernet/sing/common/json/badoption` (NOT
  `github.com/sagernet/sing/common/badoption`). The latter does not
  exist; the former is the canonical import for the option package
  (see `option/dns.go` for the established pattern).
- `badoption.Prefix` is a defined type over `netip.Prefix` with custom
  `MarshalJSON`/`UnmarshalJSON`/`Build`. To convert to a stdlib prefix
  for inspection, use `netip.Prefix(*d.Xlat464.Prefix)`. The dereference
  of a nil pointer would panic, so nil-check the pointer first.
- `UnmarshalJSON` on a `*badoption.Prefix` calls `netip.ParsePrefix`,
  which rejects empty strings and syntactically invalid CIDRs. We rely
  on this to short-circuit malformed input.

### netip prefix validation order
- For `netip.Prefix` with an IPv4-mapped address (e.g. `::ffff:c000:200`),
  `prefix.Addr().Is4In6()` is `true` and `prefix.Addr().Is6()` is also
  `true`. The validation order must check `Is4In6` BEFORE `Is6` so
  IPv4-mapped prefixes hit the dedicated "not supported" error rather
  than the generic "must be an IPv6" error.
- `prefix.Bits()` returns the *network* bit length. A `/96` IPv6 prefix
  has `Bits()==96`; `/64` has `Bits()==64`. The bit-length gate fires
  after the Is4In6 / Is6 family check, so non-IPv6 inputs never reach
  it and the message is more specific.

### DirectOutboundOptions contract
- `DirectOutboundOptions` is a type alias of `_DirectOutboundOptions`
  (NOT a wrapper). All unmarshal logic lives on the alias's
  `UnmarshalJSONContext` which calls
  `json.UnmarshalDisallowUnknownFields` first. Disallow-unknown-fields
  is the existing contract for direct options; we must not weaken it.
- A non-nil `*Xlat464Options` with a nil `*badoption.Prefix` is the
  shape produced by `{"xlat464":{}}` because `badoption.Prefix` is
  unmarshalled by its own `UnmarshalJSON` only when the JSON value is
  a quoted string. Empty object has no `prefix` key, so Prefix stays
  nil. Validating nil Pointer catches this case.

### TDD failing-first
- Writing the test before adding the field made the failure mode
  diagnostic: `options.Xlat464 undefined`. A green implementation
  becomes the only path to compilation. Stronger signal than
  per-test runtime failure because it proves the API surface change
  is exactly the one the test demanded.

## 2026-07-14 — Task 2: direct XLAT464 primitive

### Final package-internal API
- `newXLAT464AddressMapper(option.Xlat464Options) (xlat464AddressMapper, error)`
  validates and canonicalizes the `/96`; `synthesize(netip.Addr)` maps IPv4 and
  mapped-IPv4 into the prefix, while `reverse(netip.Addr)` maps only a matching
  non-mapped IPv6 address back to IPv4.
- `newXLAT464Dialer(dialer.ParallelInterfaceDialer, option.Xlat464Options)
  (*xlat464Dialer, error)` wraps all base dial and packet-listener paths. It
  additionally implements `dialer.ParallelNetworkDialer` so pre-resolved
  lists can retain/map only IPv4 candidates before the base interface sees
  them. Its serial packet path reverse-maps the returned destination address.
- `xlat464PacketConn` is an unexported `N.NetPacketConn` wrapper. It maps
  `WriteTo`/`WritePacket` targets forward, `ReadFrom`/`ReadPacket` sources
  backward, and inherits close, local-address, and deadline methods from the
  wrapped `N.NetPacketConn`.

### Interface and address details
- `dialer.ParallelInterfaceDialer` embeds `N.Dialer` (`DialContext`,
  `ListenPacket`) and adds `DialParallelInterface` plus
  `ListenSerialInterfacePacket`; both added methods take network/interface
  strategy arguments and return `net.Conn`/`net.PacketConn` respectively.
- `M.Socksaddr` exposes `Addr netip.Addr`, `Port uint16`, `Fqdn string`,
  `IsValid`, `IsIP`, family predicates, and `M.SocksaddrFrom`; copying the
  value then changing `Addr` preserves ports and domains without strings.
- `N.PacketConn` is the buffer interface (`ReadPacket`, `WritePacket`, close,
  local address, and deadlines); `N.NetPacketConn` adds standard
  `ReadFrom`/`WriteTo`. Raw `net.PacketConn` values are adapted through
  `bufio.NewPacketConn` before wrapping.
- `netip.Prefix.Contains` does not treat `::ffff:192.0.2.1` as inside
  `64:ff9b::/96`; reverse mapping still rejects `Is4In6` explicitly before
  testing containment. UDP net-address conversions use `AddrPort` and
  `net.UDPAddrFromAddrPort`, never `net.IP` or string address math.

## 2026-07-14 — Task 3: direct dialer construction

- `common/dialer.Options` now exposes the internal-only `DialerWrapper`
  (`func(ParallelInterfaceDialer) ParallelInterfaceDialer`) and
  `ForceDomainStrategyIPv4Only` hooks. Callers that leave both zero-valued
  continue through the original construction path.
- In `NewWithOptions`, the wrapper is installed directly after the
  `NewDefault` success check (currently `dialer.go:63`) and before the
  domain-resolution branch / `NewResolveDialer` call (currently line 143).
  The strategy hook is applied at `dialer.go:140`, after explicit/default
  resolver selection (including legacy fallback) and before `NewResolveDialer`.
- The resolver test fake is an `adapter.DNSRouter` placed in the service
  context with `service.ContextWith`. Its `Lookup` records
  `adapter.DNSQueryOptions` and returns `192.0.2.1`; a socket-boundary fake
  behind a mapper wrapper then observes `64:ff9b::c000:201`. Direct-constructor
  tests use the same fake-router shape with a cancelled dial context, so a
  lookup is observed without performing a real network connection.

## 2026-07-14 — Task 4: direct TCP outbound normalization

- `NewOutbound` now validates one `xlat464AddressMapper`, retains it on
  `Outbound`, and gives the existing `xlat464Dialer` wrapper that same mapper.
  Literal TCP destinations still map in the wrapper, so outbound logging keeps
  the original requested `M.Socksaddr`.
- `Outbound.DialParallel` and `Outbound.DialParallelNetwork` invoke
  `normalizeTCPDestinationAddresses` only in their TCP branches, after logging
  and before `dialer.DialParallelNetwork`. It filters supplied lists to IPv4 or
  IPv4-in-IPv6 candidates and synthesizes them; an all-AAAA supplied list fails
  before any native IPv6 socket attempt can race.
- Todo 5 must leave `DialContext`'s UDP case, `ListenPacket`, and
  `ListenSerialNetworkPacket` unchanged. The `xlat464` mapper field is now
  available on `Outbound`, but Task 4 intentionally uses it only in the two
  TCP supplied-address entry points.

## 2026-07-14 — Task 5: direct UDP normalization

- `xlat464Dialer.ListenPacket` already maps literal destinations and returns
  `xlat464PacketConn`, so `Outbound.ListenPacket` delegates without adding a
  second packet wrapper. This preserves packet deadline delegation and the
  disabled path's unwrapped connection.
- `Outbound.ListenSerialNetworkPacket` filters supplied address lists to IPv4
  candidates, synthesizes them before serial selection, and reverse-maps the
  selected address before returning it to the route layer. The route NAT
  wrapper therefore compares against the logical IPv4 destination while its
  socket-facing packet adapter uses the synthesized IPv6 destination.
- Route-level coverage includes a native AAAA candidate before the IPv4
  candidate, proving serial UDP selection cannot open a native IPv6 socket
  when XLAT464 is enabled. A prefixed reply is reverse-mapped below route NAT,
  so the application-facing source remains `192.0.2.1`.

## 2026-07-14 — Task 6: Documentation

### Docs formatter behavior
- `go run cmd/internal/format_docs/main.go` exits 0 with no warnings on the
  xlat464 docs. It did not rewrite or reformat any of the new content, which
  means the Markdown structure (headings, admonitions, JSON fences) conforms
  to the project's expected style on the first pass.
- A second run produced identical output, confirming convergence.

### Whitespace
- `git diff --check` passed cleanly. The Chinese file's original structure
  block used a blank line between `"tag"` and `"override_address"` where the
  English file used trailing spaces. The formatter normalized this; the final
  diff shows both files use consistent blank-line padding inside JSON blocks.

### Callout style decision
- Used `!!! quote "Changes in sing-box 1.14.0"` with `:material-plus:` for
  the new `xlat464` field, matching the established pattern in
  `docs/configuration/shared/dial.md`. The `!!! info "Contract"` admonition
  was chosen for the field-level contract block because it is neither a
  warning nor a deprecation but a factual behavioral guarantee.

### Chinese translation note
- Kept `"契约"` as the admonition title for "Contract" since the Chinese
  docs already use Chinese labels for admonition headings. All bullet points
  are semantically equivalent to the English version; JSON examples are
  identical.

## 2026-07-14 — Task 7: Release notes and regression suite

### Changelog style
- The `1.14.0-alpha.24` section had only a bare "Fixes and improvement" line.
  Adding a feature bullet before it follows the convention of alpha.22/alpha.21
  where feature bullets precede the "Fixes" line.
- Used a direct doc link `[direct outbound](/configuration/outbound/direct/#xlat464)`
  instead of a `**1**` footnote because xlat464 is a single option addition, not
  a multi-faceted feature requiring explanation. The linked doc page has the
  full contract.

### Docs formatter
- `go run cmd/internal/format_docs/main.go` accepts changelog bullets with
  inline Markdown links without reformatting. Two runs confirmed convergence.

## 2026-07-14 — F2 race-detector skip for TestXLAT464RouteUDP

### Race-detector skip pattern
- `TestXLAT464RouteUDP` fails deterministically under `go test -race` because
  `route.NewConnectionManager` wraps the connection in
  `sagernet/sing/common/canceler.TimeoutPacketConn`, which writes to its
  `active time.Time` field from both `ReadPacket` and `WritePacket` goroutines
  without synchronization (`packet_timeout.go:38,54`). This is a data race in
  the **upstream** `sagernet/sing` dependency, not in xlat464 code.
- All other 19 xlat464 tests pass under `-race`; only this single route-level
  integration test triggers the upstream race.
- Fix: build-tag-guarded `raceEnabled` constant (`race_on_test.go` with
  `//go:build race`, `race_off_test.go` with `//go:build !race`) plus an
  early `t.Skip` when the race detector is active. This preserves full
  production-path coverage on non-race runs while keeping CI clean under
  `-race`.

### Pre-existing test failures
- `common/tlsfragment.TestTLSFragment` fails with a TLS handshake error —
  network-dependent, unrelated to protocol/direct or dialer changes.
- `experimental/libbox` fails to build — mobile library target, not part of
  the feature's scope.
- Both were excluded from the feature claim with unchanged-baseline proof.
