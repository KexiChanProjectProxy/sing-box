# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

---

## [1.13.0.1] - 2026-02-24

### Added

- **Cloudflared outbound**: Embedded Cloudflare WARP/tunnel WebSocket transport, indistinguishable from the official client; supports native detour and optional TLS
- **LoadBalance outbound**: Full-featured load balancer with `hash`, `urltest`, and `hysteresis` strategies; eTLD+1 sticky sessions, ASN-aware routing, geosite group matching, and configurable `tolerance` for Top-N candidate pool stabilization
- **HAProxy DNS resolver**: DNS resolver with hold-TTL, retry, and stale-while-revalidate semantics (HAProxy-style)
- **Per-user IPv6 assignment**: Deterministic per-user IPv6 address derived from FNV-128a hash + configurable prefix on direct outbound
- **464XLAT (CLAT)**: RFC 6877 IPv4-in-IPv6 translation on direct outbound
- **ASN database**: MaxMind GeoLite2-ASN MMDB support for ASN-based routing rules
- **Custom geosite matcher**: User-defined geosite database path and matcher
- **AnyTLS masquerade**: File, proxy, string, and redirect masquerade modes; extended session pool options (`ensure_idle_session`, `heartbeat`, `max_connection_lifetime`, and more)
- **Multi-output logging**: Simultaneous JSON, HTTP-batch, and formatted log outputs
- **Transparent proxy extensions**: `use_origin_dst` / `revert_origin_dst` flags for TPROXY/redirect inbounds
- **Route options**: `sniff_override_destination` as a per-route option; `default_tcp_keep_alive` and `default_tcp_keep_alive_interval` on `route`
- **MatchedRuleSet propagation**: Matched rule-set tags forwarded to load-balance hash for deterministic session affinity

### Changed

- Rebased onto upstream sing-box **1.13.0-rc.6**, adopting all upstream architectural changes, API updates, and bug fixes
- All 1.12.x custom features re-implemented and adapted to the new 1.13.0 adapter/router architecture

### Fixed

- Cloudflared `Read()` EOF leak and missing deadline methods (backported fix)
- AnyTLS TLS now optional for outbound connections

---

## [1.12.14.30] - 2026-02-18

### Added

### Changed

### Deprecated

### Removed

### Fixed

### Security

---

## [1.12.14.29] - 2026-02-18

### Added

### Changed

### Deprecated

### Removed

### Fixed

### Security

### Features

- **LoadBalance**: Added configurable `tolerance` field for Top-N candidate pool stabilization
  - Previous Top-N candidates within `tolerance` ms of the N-th ranked node remain eligible
  - Reduces hash ring rebuilds for `consistent_hash` strategy
  - Prevents sticky session disruptions when nodes have similar latencies
  - Default: `0` (disabled, backward compatible)

### Technical Details

- Added `Tolerance uint16` field to `LoadBalanceOutboundOptions`
- Modified `selectTopN()` to accept previous candidate tags and apply tolerance logic
- Added 6 comprehensive unit tests covering all edge cases
- Updated documentation in English and Chinese

## [1.12.14.28] - 2026-02-18

### Features

- **DNS**: HAProxy-style DNS resolver implementation
