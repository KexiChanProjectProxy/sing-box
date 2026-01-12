# sing-box Feature Additions Plan

> **Purpose**: High-level plan documenting features added to sing-box
>
> **Audience**: Claude Code working on sing-box projects
>
> **Format**: Descriptive plan without code implementation details
>
> **Period**: 18 commits from Jan 2025

---

## Overview

This document describes features added to sing-box in three major areas:

1. **LoadBalance Outbound**: URL-test driven load balancing with HAProxy-style backup tiers
2. **AnyTLS Session Management**: Advanced connection pool management and lifecycle control
3. **System Improvements**: TCP keep-alive and hash-based routing utilities

---

## 1. LoadBalance Outbound

### Core Concept

A new outbound type that distributes connections across multiple healthy outbounds based on URL-test latency results, with automatic failover to backup tier.

### Features Added

#### 1.1 URL-Test Driven Selection
- Health check all configured outbounds using URL test probes
- Rank outbounds by latency (low to high)
- Select Top-N fastest outbounds from each tier
- Store health check history for monitoring

#### 1.2 Two-Tier System (Primary + Backup)
- **Primary tier**: Main outbounds used during normal operation
- **Backup tier**: Fallback outbounds activated when primaries fail
- **HAProxy semantics**: Backups only used when ALL primaries unavailable
- Top-N selection per tier (configurable independently)

#### 1.3 Load Balancing Strategies

**Random Strategy**:
- Uniform random distribution across available candidates
- No session affinity
- Simple and fast

**Consistent Hash Strategy**:
- Session affinity: same input → same outbound
- Virtual nodes for better distribution
- Minimal remapping when pool changes (less than 33%)
- Configurable hash key composition

#### 1.4 Hash Key Composition (10 Parts)

Ability to build composite hash keys from metadata:

1. **src_ip**: Source IP address
2. **dst_ip**: Destination IP address
3. **dst_port**: Destination port number
4. **network**: Network type (TCP/UDP)
5. **domain**: Full destination domain
6. **inbound_tag**: Inbound connection tag
7. **matched_ruleset**: Matched ruleset tag (NEW)
8. **etld_plus_one**: Top-level domain extraction (NEW)
9. **matched_ruleset_or_etld**: Smart fallback mode (NEW)
10. **salt**: Custom randomization string

#### 1.5 Smart Routing Mode: matched_ruleset_or_etld

Priority-based hash key selection:
- If connection matches a ruleset: use ruleset tag for hashing
- If no ruleset matches: fall back to eTLD+1 domain hashing
- Unified configuration for both rule-based and direct connections
- Maintains session stickiness across mixed traffic patterns

#### 1.6 Hysteresis (Flapping Prevention)

Tier switching requires sustained failures to prevent oscillation:
- Track consecutive primary failure count
- Require N failures before activating backup tier (default: 3)
- Hold backup tier for minimum time before switching back (default: 30s)
- Prevents rapid back-and-forth switching

#### 1.7 Bootstrap Mode

Immediate connection handling before first health check completes:
- Use all primary outbounds with equal weight
- No startup delay waiting for health checks
- Respects configured strategy (random or consistent_hash)
- Automatic transition to health-based selection after first check
- Fixes "no candidates available (not initialized)" startup error

#### 1.8 Clash API Compatibility

URLTest method for on-demand health checking:
- Enables Clash API `/proxies/:name/delay` endpoint
- Manual health check triggering via API
- Returns latency map for all outbounds
- Real-time monitoring integration

#### 1.9 Empty Pool Handling

Two behaviors when no candidates available:
- **error**: Return error (fail closed)
- **fallback_all**: Use all configured outbounds (fail open)

#### 1.10 Network Filtering

Automatic filtering by connection type:
- TCP connections: only use TCP-capable outbounds
- UDP connections: only use UDP-capable outbounds

---

## 2. AnyTLS Session Management

### Core Concept

Advanced idle session pool management for maintaining warm connections with fine-grained lifecycle control.

### Features Added

#### 2.1 Proactive Pool Maintenance (ensure_idle_session)

Active session creation to maintain minimum pool size:
- Automatically creates new sessions when pool drops below threshold
- Asynchronous creation to avoid blocking
- Different from passive protection (min_idle_session)
- Pre-warmed connections for reduced latency

**Purpose**: High availability and low-latency connection establishment

#### 2.2 Rate-Limited Pool Creation (ensure_idle_session_create_rate)

Limits session creation speed to prevent connection storms:
- Maximum N sessions created per cleanup cycle
- Gradual pool recovery spreads load over time
- Protects destination servers from sudden spikes
- Default unlimited (0) for backward compatibility

**Use Cases**:
- Rate-limited destinations: slow creation (1-3 per cycle)
- Balanced approach: moderate creation (3-5 per cycle)
- Fast recovery: quick creation (5-10 per cycle)

#### 2.3 Age-Based Connection Rotation (max_connection_lifetime)

Automatic connection expiration based on age:
- Track session creation timestamp
- Close sessions exceeding maximum lifetime
- Oldest sessions closed first
- Works with proactive creation for automatic rotation

**Benefits**:
- Security: limit connection exposure window
- Load balancing: distribute across dynamic backends
- NAT resilience: periodic session renewal
- Prevent indefinite connection lifetime

#### 2.4 Lifetime Randomization (connection_lifetime_jitter)

Randomize connection lifetimes to prevent thundering herd:
- Each session gets random lifetime: base ± jitter
- Distributes expiration across time window
- Prevents simultaneous reconnection spikes
- Smooth connection rotation over time

**Example**: 1h base with 15m jitter = 45-75 minute lifetime range

#### 2.5 Independent Age Protection (min_idle_session_for_age)

Separate minimum session configuration for age vs idle cleanup:
- **min_idle_session**: Protects from idle timeout closure
- **min_idle_session_for_age**: Protects from age-based closure
- Independent control for different cleanup scenarios

**Use Cases**:
- Aggressive age rotation + generous idle protection
- Conservative age rotation + aggressive idle cleanup
- Balanced protection for both scenarios

#### 2.6 Idle Cleanup Logging

Debug logging for idle timeout cleanup:
- Summary: sessions found, closing count, keeping count
- Per-session details: sequence number, idle duration, timestamp
- Matches age cleanup logging format
- Fixes monitoring blindspot (was completely silent)

---

## 3. System Improvements

### 3.1 TCP Keep-Alive System Defaults

Enable TCP keep-alive using operating system defaults:
- Removed hardcoded values (10min idle, 75sec interval)
- Use system-level TCP keep-alive settings
- More adaptive to deployment environments
- Applied to both inbound and outbound connections

**Benefits**:
- Better NAT traversal
- Improved connection resilience
- Follows OS tuning
- Consistent behavior across platforms

### 3.2 Go 1.23+ Keep-Alive Compatibility

Fix TCP keep-alive for Go 1.23+ versions:
- Go 1.23 changed keep-alive API structure
- Updated to use new KeepAliveConfig properly
- Maintain backward compatibility with older Go versions
- Ensure keep-alive works across all Go releases

### 3.3 eTLD+1 Domain Extraction

Extract top-level domain using Public Suffix List:
- Handle multi-part TLDs (co.uk, gov.uk, ac.jp)
- Domain normalization (lowercase, strip ports, trailing dots)
- IP address detection (return placeholder)
- Used for consistent hash routing

**Examples**:
- `a.b.example.com` → `example.com`
- `a.b.example.co.uk` → `example.co.uk` (not `co.uk`)
- `192.168.1.1` → `-` (IP placeholder)

### 3.4 Ruleset Matching Integration

Track which ruleset matched a connection:
- Added MatchedRuleSet field to connection metadata
- Populated during route rule evaluation
- Available for hash key construction
- Enables ruleset-based load balancing

---

## 4. Documentation Added

### 4.1 LoadBalance Documentation

Comprehensive documentation in English and Chinese:
- Complete field reference with defaults
- Top-N selection explanation
- Load balancing strategy comparison
- Hash-based routing with all 10 key parts
- matched_ruleset_or_etld deep dive
- 6 complete configuration examples
- Best practices and troubleshooting
- Technical implementation details

### 4.2 AnyTLS Documentation

Session management documentation in English and Chinese:
- All new field descriptions
- Session Pool Management Guide
- Configuration examples for each feature
- Use case scenarios with recommendations
- Debug logging examples
- Interaction between features explained

### 4.3 Configuration Examples

Five LoadBalance example configurations:
1. Simple random load balancing
2. Consistent hash with virtual nodes
3. Per-source IP + ruleset routing
4. Per-source IP + top domain routing
5. Smart ruleset-or-domain fallback routing

---

## 5. Testing Coverage

### 5.1 LoadBalance Unit Tests

Nine comprehensive test functions:
- Top-N selection validation
- Backup tier activation logic
- Consistent hash stability and distribution
- Hysteresis flapping prevention
- Empty pool behavior (error vs fallback)
- Concurrent access and thread safety
- Network filtering (TCP vs UDP)
- Random distribution uniformity
- Hash key construction from metadata

### 5.2 Hash Key Tests

Fourteen test functions covering:
- Basic key parts (IP, port, network, domain)
- Advanced key parts (inbound_tag, salt)
- Composite keys (src_ip + ruleset, src_ip + etld)
- Smart fallback mode (ruleset_or_etld)
- Empty value handling and placeholders
- Hash consistency across calls
- Backward compatibility verification

### 5.3 eTLD Extraction Tests

Four test functions validating:
- Standard domain extraction
- Multi-part TLD handling via PSL
- IP address detection (IPv4 and IPv6)
- Edge cases and normalization
- Malformed input handling

### 5.4 Race Detector

All tests pass with race detector enabled:
- Zero data races detected
- Thread-safe atomic operations validated
- Concurrent access patterns verified

---

## 6. Development Timeline

### Phase 1: AnyTLS Session Management
1. ensure_idle_session - Proactive pool maintenance
2. max_connection_lifetime - Age-based rotation
3. connection_lifetime_jitter + min_idle_session protection
4. min_idle_session_for_age - Separate age protection
5. ensure_idle_session_create_rate - Rate limiting

### Phase 2: TCP Keep-Alive
6. Go 1.23+ compatibility fix
7. Binary rebuild with fix
8. System-wide keep-alive defaults

### Phase 3: LoadBalance Foundation
9. LoadBalance outbound implementation
10. Composite hash keys (eTLD+1, ruleset)
11. matched_ruleset_or_etld smart mode

### Phase 4: LoadBalance Polish
12. Comprehensive documentation (EN + CN)
13. Synchronous initialization attempt
14. Bootstrap mode (async with fallback)
15. URLTest Clash API compatibility

### Phase 5: Monitoring & Logging
16. Idle cleanup debug logging
17. Submodule update for metrics support

---

## 7. Feature Integration

### How Features Work Together

**LoadBalance + Hash Routing**:
- LoadBalance uses eTLD extraction for domain-based routing
- Ruleset matching provides session affinity by category
- Smart fallback combines both approaches

**AnyTLS + LoadBalance**:
- LoadBalance distributes across multiple AnyTLS outbounds
- AnyTLS maintains warm connection pools per outbound
- Combined: High availability with low latency

**Session Management Features**:
- ensure_idle_session + max_connection_lifetime = automatic rotation
- connection_lifetime_jitter prevents thundering herd during rotation
- Separate min protections for idle vs age cleanup
- Rate limiting prevents storms during pool recovery

**TCP Keep-Alive + AnyTLS**:
- System keep-alive maintains idle connections
- AnyTLS manages pool with proactive creation
- Combined: Resilient NAT traversal with availability

---

## 8. Key Improvements

### Performance
- Bootstrap mode eliminates startup delay
- Pre-warmed connection pools reduce latency
- Consistent hashing minimizes remapping overhead
- Async health checks don't block traffic

### Reliability
- Hysteresis prevents tier flapping
- Backup tier provides automatic failover
- Minimum session protection maintains availability
- Rate limiting prevents connection storms

### Flexibility
- 10 configurable hash key parts
- Independent cleanup protections
- Two load balancing strategies
- Configurable empty pool behavior

### Observability
- Comprehensive debug logging
- URLTest API for monitoring
- Health check history storage
- Per-session closure tracking

### Compatibility
- Clash API integration
- Go version compatibility (1.23+ fixes)
- Backward compatible defaults
- System-level TCP settings

---

## 9. Statistics

**Code Additions**:
- Total commits: 18
- Files modified: ~50
- Files added: ~20
- Lines added: ~6,000+ (including tests and docs)

**Components**:
- LoadBalance implementation: ~1,100 lines
- LoadBalance tests: ~900 lines
- Documentation: ~1,100 lines (EN + CN)
- Hash routing utilities: ~600 lines
- AnyTLS enhancements: ~500 lines

**Testing**:
- Unit tests: 26 functions
- Integration tests: 3 functions
- All tests passing
- Race detector clean

---

## 10. Configuration Fields Added

### LoadBalance Outbound Options

**Basic Configuration**:
- `primary_outbounds`: Primary tier outbound tags
- `backup_outbounds`: Backup tier outbound tags (optional)
- `strategy`: "random" | "consistent_hash"
- `link`: URL for health check probes
- `interval`: Health check interval
- `timeout`: Health check timeout
- `idle_timeout`: Connection idle timeout

**Top-N Selection**:
- `top_n.primary`: Top-N for primary tier
- `top_n.backup`: Top-N for backup tier

**Hash Configuration**:
- `hash.key_parts`: Array of metadata parts for key
- `hash.virtual_nodes`: Virtual nodes per outbound
- `hash.on_empty_key`: "random" | "hash_empty"
- `hash.salt`: Custom salt string

**Hysteresis**:
- `hysteresis.primary_failures`: Failure count threshold
- `hysteresis.backup_hold_time`: Minimum backup duration

**Empty Pool**:
- `empty_pool_action`: "error" | "fallback_all"

### AnyTLS Outbound Options

**Session Pool**:
- `ensure_idle_session`: Target pool size (proactive)
- `min_idle_session`: Minimum for idle protection (passive)
- `min_idle_session_for_age`: Minimum for age protection (passive)

**Pool Creation**:
- `ensure_idle_session_create_rate`: Max sessions per cycle

**Lifecycle**:
- `max_connection_lifetime`: Maximum session age
- `connection_lifetime_jitter`: Randomization range
- `idle_session_timeout`: Idle timeout duration
- `idle_session_check_interval`: Cleanup cycle interval

---

## 11. Use Cases

### LoadBalance

**Scenario 1: Geographic Load Distribution**
- Primary: Low-latency regional proxies
- Backup: Global fallback proxies
- Strategy: consistent_hash with src_ip
- Result: Users stick to regional proxy, fall back on failure

**Scenario 2: Service Category Routing**
- Hash key: ["src_ip", "matched_ruleset"]
- Rulesets: streaming, gaming, general
- Result: Each service type routes to optimized outbound

**Scenario 3: CDN Optimization**
- Hash key: ["src_ip", "matched_ruleset_or_etld"]
- Rulesets: major CDNs (Netflix, YouTube)
- Result: CDN traffic routes by ruleset, others by domain

### AnyTLS

**Scenario 1: High Availability Service**
- ensure_idle_session: 10
- min_idle_session: 5
- max_connection_lifetime: 2h
- Result: Always-ready pool with periodic rotation

**Scenario 2: Rate-Limited Destination**
- ensure_idle_session: 5
- ensure_idle_session_create_rate: 1
- Result: Gradual pool recovery, no rate limit triggers

**Scenario 3: Security-Sensitive Connection**
- max_connection_lifetime: 30m
- connection_lifetime_jitter: 10m
- min_idle_session_for_age: 2
- Result: Regular rotation (20-40m), maintains minimum pool

---

## End of Plan

This document serves as a reference for Claude Code working on sing-box projects. It describes **what** was added without implementation details, suitable for understanding the feature landscape and planning similar additions.

