# Feature Migration Guide: sing-box 1.12.14.x → 1.13.x

**Document Version:** 1.0
**Target Audience:** Future AI implementer / Developer
**Purpose:** Comprehensive guide to migrate new features from 1.12.14.x to 1.13.x

---

## Table of Contents

1. [Overview](#overview)
2. [AnyTLS Outbound Enhancements](#1-anytls-outbound-enhancements)
3. [VLESS Connection Pool](#2-vless-connection-pool)
4. [LoadBalance Outbound Features](#3-loadbalance-outbound-features)
5. [TieredLoadBalance Outbound (NEW)](#4-tieredloadbalance-outbound-new)
6. [Sniff Override Destination](#5-sniff-override-destination)
7. [Implementation Checklist](#implementation-checklist)
8. [Testing Strategy](#testing-strategy)

---

## Overview

This document describes **5 major feature areas** added to sing-box 1.12.14.x that need to be migrated to 1.13.x:

| Feature | Priority | Complexity | Impact |
|---------|----------|------------|--------|
| AnyTLS Enhancements | Medium | Low | Session management |
| VLESS Connection Pool | High | High | Performance boost |
| LoadBalance Features | High | Medium | Advanced routing |
| TieredLoadBalance | High | High | Auto-tier selection |
| Sniff Override | Low | Low | QoL improvement |

**Migration Strategy:**
- Start with low-complexity features (AnyTLS, Sniff Override)
- Then implement medium-complexity (LoadBalance)
- Finally tackle high-complexity (VLESS Pool, TieredLoadBalance)

---

## 1. AnyTLS Outbound Enhancements

### 1.1 Overview

Two new configuration options were added to AnyTLS outbound for better session pool management:

1. **`heartbeat`**: TCP-level keepalive for early connection failure detection
2. **`ensure_idle_session_create_rate`**: Rate limiting for session pool recovery

### 1.2 Configuration Schema

**File: `option/anytls.go`**

```go
type AnyTLSOutboundOptions struct {
    DialerOptions
    ServerOptions
    OutboundTLSOptionsContainer
    Password                     string             `json:"password,omitempty"`
    IdleSessionCheckInterval     badoption.Duration `json:"idle_session_check_interval,omitempty"`
    IdleSessionTimeout           badoption.Duration `json:"idle_session_timeout,omitempty"`
    MinIdleSession               int                `json:"min_idle_session,omitempty"`
    MinIdleSessionForAge         int                `json:"min_idle_session_for_age,omitempty"`
    EnsureIdleSession            int                `json:"ensure_idle_session,omitempty"`

    // NEW: Rate limiting for session creation
    EnsureIdleSessionCreateRate  int                `json:"ensure_idle_session_create_rate,omitempty"`

    // NEW: TCP keepalive interval
    Heartbeat                    badoption.Duration `json:"heartbeat,omitempty"`

    MaxConnectionLifetime        badoption.Duration `json:"max_connection_lifetime,omitempty"`
    ConnectionLifetimeJitter     badoption.Duration `json:"connection_lifetime_jitter,omitempty"`
}
```

### 1.3 Implementation Details

#### 1.3.1 Heartbeat Option

**Purpose:** Enable TCP-level keepalive to detect dead connections early.

**Pass-through to sing-anytls:**
- The option is stored in config but passed directly to the underlying `sing-anytls` library
- No logic in sing-box itself, just configuration plumbing
- Requires `sing-anytls` library support (KexiChanProjectProxy fork v0.0.12+)

**File: `protocol/anytls/outbound.go`**

```go
// In NewOutbound() function, pass heartbeat to client config
client, err := anytls.NewClient(anytls.ClientConfig{
    // ... other options ...
    Heartbeat: time.Duration(options.Heartbeat), // NEW
})
```

#### 1.3.2 Ensure Idle Session Create Rate

**Purpose:** Prevent connection storms when pool needs to create many sessions simultaneously.

**Behavior:**
- Limits how many new sessions can be created per cleanup cycle
- Spreads pool recovery over multiple cycles
- Protects destination servers from sudden load spikes
- Default: 0 (unlimited, backward compatible)

**Recommended Values:**
- Sensitive destinations: `1-3` (slow recovery)
- Balanced approach: `3-5` (recommended)
- Fast recovery: `5-10` (stable destinations)
- Unlimited: `0` (maximum availability)

**Pass-through to sing-anytls:**
```go
client, err := anytls.NewClient(anytls.ClientConfig{
    // ... other options ...
    EnsureIdleSessionCreateRate: options.EnsureIdleSessionCreateRate, // NEW
})
```

### 1.4 Configuration Example

```json
{
    "type": "anytls",
    "tag": "anytls-out",
    "server": "example.com",
    "server_port": 443,
    "password": "your-password",

    "idle_session_check_interval": "30s",
    "ensure_idle_session": 10,
    "ensure_idle_session_create_rate": 2,
    "heartbeat": "15s",

    "max_connection_lifetime": "10m",
    "connection_lifetime_jitter": "2m"
}
```

**Recovery Example:**
- Pool drops to 0 sessions
- Creates 2 new sessions per 30s cycle
- Full recovery (10 sessions) in ~2.5 minutes

### 1.5 Migration Steps

1. **Update option schema** (`option/anytls.go`)
   - Add `EnsureIdleSessionCreateRate int`
   - Add `Heartbeat badoption.Duration`

2. **Pass values to sing-anytls** (`protocol/anytls/outbound.go`)
   - Include both options in client config struct
   - No additional logic needed

3. **Update sing-anytls dependency**
   - Ensure using KexiChanProjectProxy fork v0.0.12+
   - Or merge upstream if feature was upstreamed

4. **Documentation**
   - Add to `docs/configuration/outbound/anytls.md`
   - Include recovery time calculation examples
   - Document recommended values by use case

### 1.6 Testing

```bash
# Test heartbeat detection
# 1. Establish connection
# 2. Block network suddenly
# 3. Verify heartbeat detects dead connection within 2× heartbeat interval

# Test rate limiting
# 1. Set ensure_idle_session=20, rate=2
# 2. Kill all sessions
# 3. Monitor creation: should create 2 per cycle
# 4. Verify gradual recovery over multiple cycles
```

---

## 2. VLESS Connection Pool

### 2.1 Overview

**Major Feature:** AnyTLS-style connection pool management for VLESS protocol (client-side only).

**Benefits:**
- ⚡ **Reduced latency**: Pre-established TCP+TLS connections
- 🔄 **Improved reliability**: Proactive connection lifecycle management
- 🛡️ **Dead connection detection**: TCP keepalive + age rotation
- 📊 **Network-aware**: Auto-reset on interface changes

**Architecture Pattern:** Mirrors AnyTLS pool design for consistency.

### 2.2 Configuration Schema

**File: `option/vless.go`**

```go
type VLESSConnectionPoolOptions struct {
    // Pool sizing
    EnsureIdleSession           int                `json:"ensure_idle_session,omitempty"`
    EnsureIdleSessionCreateRate int                `json:"ensure_idle_session_create_rate,omitempty"`
    MinIdleSession              int                `json:"min_idle_session,omitempty"`
    MinIdleSessionForAge        int                `json:"min_idle_session_for_age,omitempty"`

    // Timeouts
    IdleSessionCheckInterval badoption.Duration `json:"idle_session_check_interval,omitempty"`
    IdleSessionTimeout       badoption.Duration `json:"idle_session_timeout,omitempty"`

    // Lifetime rotation
    MaxConnectionLifetime    badoption.Duration `json:"max_connection_lifetime,omitempty"`
    ConnectionLifetimeJitter badoption.Duration `json:"connection_lifetime_jitter,omitempty"`

    // Heartbeat (TCP-level keepalive, not VLESS protocol)
    Heartbeat badoption.Duration `json:"heartbeat,omitempty"`
}

type VLESSOutboundOptions struct {
    DialerOptions
    ServerOptions
    UUID    string      `json:"uuid"`
    Flow    string      `json:"flow,omitempty"`
    Network NetworkList `json:"network,omitempty"`
    OutboundTLSOptionsContainer
    Multiplex      *OutboundMultiplexOptions   `json:"multiplex,omitempty"`
    Transport      *V2RayTransportOptions      `json:"transport,omitempty"`
    PacketEncoding *string                     `json:"packet_encoding,omitempty"`

    // NEW: Connection pool configuration
    ConnectionPool *VLESSConnectionPoolOptions `json:"connection_pool,omitempty"`
}
```

### 2.3 Implementation Architecture

**File: `protocol/vless/pool.go`** (441 lines, NEW file)

#### 2.3.1 Core Components

```go
// ConnectionPool manages pre-established VLESS connections
type ConnectionPool struct {
    mu      sync.RWMutex
    ctx     context.Context
    cancel  context.CancelFunc
    dialer  N.Dialer

    // Configuration
    serverAddr         M.Socksaddr
    uuid               string
    tlsConfig          *tls.Config
    ensureIdleSession  int
    createRate         int
    minIdleSession     int
    minIdleSessionAge  int
    checkInterval      time.Duration
    idleTimeout        time.Duration
    maxLifetime        time.Duration
    lifetimeJitter     time.Duration
    heartbeatInterval  time.Duration

    // Connection pool
    connections []*pooledConnection

    // Metrics
    totalCreated   atomic.Uint64
    totalReused    atomic.Uint64
    totalExpired   atomic.Uint64
    totalIdle      atomic.Uint64
}

type pooledConnection struct {
    conn       net.Conn
    createdAt  time.Time
    lastUsed   time.Time
    expiresAt  time.Time  // For lifetime rotation
    inUse      bool
}
```

#### 2.3.2 Lifecycle Management

```go
// 1. Pool Initialization
func NewConnectionPool(ctx, options) *ConnectionPool {
    pool := &ConnectionPool{...}

    // Start background goroutines
    go pool.cleanupLoop()       // Periodic cleanup
    go pool.ensureIdleLoop()    // Ensure minimum pool size
    if heartbeat > 0 {
        go pool.heartbeatLoop() // TCP keepalive
    }

    return pool
}

// 2. Connection Acquisition
func (p *ConnectionPool) GetConnection(ctx) (net.Conn, error) {
    p.mu.Lock()

    // Try to reuse existing idle connection
    for i, pc := range p.connections {
        if !pc.inUse && !pc.isExpired() {
            pc.inUse = true
            pc.lastUsed = time.Now()
            p.mu.Unlock()

            p.totalReused.Add(1)
            return pc.conn, nil
        }
    }
    p.mu.Unlock()

    // No idle connection, create new one
    return p.createConnection(ctx)
}

// 3. Connection Return
func (p *ConnectionPool) ReturnConnection(conn net.Conn) {
    p.mu.Lock()
    defer p.mu.Unlock()

    for _, pc := range p.connections {
        if pc.conn == conn {
            pc.inUse = false
            pc.lastUsed = time.Now()
            return
        }
    }
}

// 4. Cleanup Loop
func (p *ConnectionPool) cleanupLoop() {
    ticker := time.NewTicker(p.checkInterval)
    defer ticker.Stop()

    for {
        select {
        case <-p.ctx.Done():
            return
        case <-ticker.C:
            p.cleanup()
        }
    }
}

func (p *ConnectionPool) cleanup() {
    p.mu.Lock()
    defer p.mu.Unlock()

    now := time.Now()
    kept := make([]*pooledConnection, 0, len(p.connections))

    for _, pc := range p.connections {
        if pc.inUse {
            kept = append(kept, pc)
            continue
        }

        // Check idle timeout
        if p.idleTimeout > 0 && now.Sub(pc.lastUsed) > p.idleTimeout {
            // Protect minimum pool size
            if len(kept) >= p.minIdleSession {
                pc.conn.Close()
                p.totalExpired.Add(1)
                continue
            }
        }

        // Check age-based expiration
        if pc.expiresAt.Before(now) {
            // Protect minimum for age rotation
            if len(kept) >= p.minIdleSessionAge {
                pc.conn.Close()
                p.totalExpired.Add(1)
                continue
            }
        }

        kept = append(kept, pc)
    }

    p.connections = kept
}

// 5. Ensure Idle Loop
func (p *ConnectionPool) ensureIdleLoop() {
    ticker := time.NewTicker(p.checkInterval)
    defer ticker.Stop()

    for {
        select {
        case <-p.ctx.Done():
            return
        case <-ticker.C:
            p.ensureIdle()
        }
    }
}

func (p *ConnectionPool) ensureIdle() {
    p.mu.RLock()
    currentIdle := p.countIdle()
    p.mu.RUnlock()

    if currentIdle >= p.ensureIdleSession {
        return
    }

    needed := p.ensureIdleSession - currentIdle

    // Apply rate limiting
    if p.createRate > 0 && needed > p.createRate {
        needed = p.createRate
    }

    // Create connections concurrently
    var wg sync.WaitGroup
    for i := 0; i < needed; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            _, _ = p.createConnection(p.ctx)
        }()
    }
    wg.Wait()
}

// 6. Heartbeat Loop
func (p *ConnectionPool) heartbeatLoop() {
    ticker := time.NewTicker(p.heartbeatInterval)
    defer ticker.Stop()

    for {
        select {
        case <-p.ctx.Done():
            return
        case <-ticker.C:
            p.heartbeat()
        }
    }
}

func (p *ConnectionPool) heartbeat() {
    p.mu.RLock()
    connections := make([]*pooledConnection, len(p.connections))
    copy(connections, p.connections)
    p.mu.RUnlock()

    for _, pc := range connections {
        if pc.inUse {
            continue
        }

        // Try to read with deadline
        pc.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
        buf := make([]byte, 1)
        _, err := pc.conn.Read(buf)
        pc.conn.SetReadDeadline(time.Time{})

        if err != nil && err != io.EOF {
            // Connection dead, remove it
            p.removeConnection(pc)
        }
    }
}
```

### 2.4 Integration with VLESS Outbound

**File: `protocol/vless/outbound.go`**

```go
type Outbound struct {
    // ... existing fields ...

    // NEW: Connection pool
    pool *ConnectionPool
}

func NewOutbound(ctx, router, logger, tag, options) (*Outbound, error) {
    outbound := &Outbound{
        // ... existing initialization ...
    }

    // NEW: Initialize connection pool if configured
    if options.ConnectionPool != nil {
        poolOpts := options.ConnectionPool

        // Validate: pool incompatible with TCP Fast Open
        if outbound.dialerOptions.TCPFastOpen {
            return nil, E.New("connection_pool is incompatible with tcp_fast_open")
        }

        outbound.pool = NewConnectionPool(
            ctx,
            outbound.dialer,
            outbound.serverAddr,
            options.UUID,
            outbound.tlsConfig,
            poolOpts,
        )
    }

    return outbound, nil
}

func (h *Outbound) DialContext(ctx, network, destination) (net.Conn, error) {
    // ... metadata handling ...

    var conn net.Conn
    var err error

    // NEW: Use pool if available
    if h.pool != nil && network == N.NetworkTCP {
        conn, err = h.pool.GetConnection(ctx)
        if err != nil {
            return nil, err
        }

        // Wrap connection to return to pool on close
        conn = &pooledVLESSConn{
            Conn: conn,
            pool: h.pool,
        }
    } else {
        // Original direct dial
        conn, err = h.dialer.DialContext(ctx, network, h.serverAddr)
        if err != nil {
            return nil, err
        }
    }

    // ... rest of VLESS protocol setup ...
}

// Wrapper to return connection to pool on close
type pooledVLESSConn struct {
    net.Conn
    pool *ConnectionPool
    once sync.Once
}

func (c *pooledVLESSConn) Close() error {
    c.once.Do(func() {
        c.pool.ReturnConnection(c.Conn)
    })
    return nil
}
```

### 2.5 Configuration Example

```json
{
    "type": "vless",
    "tag": "vless-out",
    "server": "example.com",
    "server_port": 443,
    "uuid": "your-uuid-here",

    "tls": {
        "enabled": true,
        "server_name": "example.com"
    },

    "connection_pool": {
        "ensure_idle_session": 10,
        "ensure_idle_session_create_rate": 3,
        "min_idle_session": 3,
        "min_idle_session_for_age": 2,

        "idle_session_check_interval": "30s",
        "idle_session_timeout": "5m",

        "max_connection_lifetime": "10m",
        "connection_lifetime_jitter": "2m",

        "heartbeat": "30s"
    }
}
```

### 2.6 Compatibility Matrix

| Feature | Compatible | Notes |
|---------|-----------|-------|
| TLS | ✅ Yes | Recommended |
| Multiplex | ✅ Yes | Works with pool |
| All transports | ✅ Yes | TCP, WS, gRPC, HTTP/2, etc. |
| TCP Fast Open | ❌ No | Validation added, returns error |
| UDP | ⚠️ N/A | Pool only for TCP, UDP uses direct dial |

### 2.7 Migration Steps

1. **Add option schema** (`option/vless.go`)
   - Create `VLESSConnectionPoolOptions` struct
   - Add `ConnectionPool *VLESSConnectionPoolOptions` to `VLESSOutboundOptions`

2. **Implement connection pool** (`protocol/vless/pool.go` - NEW FILE)
   - Copy entire pool implementation (441 lines)
   - Core: `ConnectionPool`, `pooledConnection` types
   - Methods: GetConnection, ReturnConnection, cleanup loops

3. **Integrate with outbound** (`protocol/vless/outbound.go`)
   - Add `pool *ConnectionPool` field to `Outbound` struct
   - Initialize pool in `NewOutbound` if config present
   - Add TCP Fast Open validation
   - Modify `DialContext` to use pool for TCP connections
   - Implement `pooledVLESSConn` wrapper

4. **Add comprehensive tests** (`protocol/vless/pool_test.go` - NEW FILE)
   - 18 test cases covering all scenarios
   - Race detector validation (`go test -race`)

5. **Documentation**
   - Configuration guide with examples
   - Performance benchmarks
   - Compatibility matrix

### 2.8 Testing Strategy

**File: `protocol/vless/pool_test.go`** (669 lines)

```go
func TestConnectionPool(t *testing.T) {
    // Test cases:
    // 1. Pool creation and initialization
    // 2. Connection acquisition and reuse
    // 3. Connection return to pool
    // 4. Idle timeout cleanup
    // 5. Age-based rotation
    // 6. Min idle protection
    // 7. Ensure idle session creation
    // 8. Create rate limiting
    // 9. TCP heartbeat detection
    // 10. Network change detection
    // 11. Concurrent access (race conditions)
    // 12. Pool shutdown and cleanup
    // 13. Metrics accuracy
    // 14. TCP Fast Open validation
    // 15. Multiplex compatibility
    // 16. TLS integration
    // 17. All transport types
    // 18. Error handling and edge cases
}
```

**Run tests:**
```bash
# Unit tests
go test -v ./protocol/vless -run TestConnectionPool

# Race detector
go test -race ./protocol/vless

# Integration tests
go test -v ./protocol/vless/integration_test.go
```

---

## 3. LoadBalance Outbound Features

### 3.1 Overview

Enhanced LoadBalance outbound with advanced hash-based routing options for intelligent traffic distribution.

**New Hash Key Parts:**
1. `matched_ruleset` - Route by ruleset that matched
2. `etld_plus_one` - Route by top-level domain (e.g., example.com)
3. `matched_ruleset_or_etld` - Smart fallback (ruleset → eTLD+1)
4. `dst_asn` - Route by destination ASN (ISP/CDN grouping)
5. `dst_geosite` - Route by geosite category (service grouping)

### 3.2 Configuration Schema

**File: `option/group.go`**

```go
type LoadBalanceHashOptions struct {
    KeyParts     []string `json:"key_parts,omitempty"`
    VirtualNodes int      `json:"virtual_nodes,omitempty"`
    OnEmptyKey   string   `json:"on_empty_key,omitempty"`
    KeySalt      string   `json:"key_salt,omitempty"`
}

// Supported key_parts values:
// - "src_ip": Source IP address
// - "dst_ip": Destination IP address
// - "src_port": Source port number
// - "dst_port": Destination port number
// - "network": Network type (tcp/udp)
// - "domain": Full destination domain name
// - "inbound_tag": Tag of the inbound
// - "matched_ruleset": Tag of the ruleset that matched (NEW)
// - "etld_plus_one": eTLD+1 domain suffix (NEW)
// - "matched_ruleset_or_etld": Smart fallback (NEW)
// - "dst_asn": Destination ASN (NEW, requires ASN database)
// - "dst_geosite": Destination geosite (NEW, requires geosite database)
```

### 3.3 Implementation Details

**File: `protocol/group/loadbalance.go`**

#### 3.3.1 Matched Ruleset

**Purpose:** Route connections based on which rule matched them.

**Use Case:** Different outbounds for different content categories.

```go
case "matched_ruleset":
    if metadata.MatchedRuleSet != "" {
        parts = append(parts, metadata.MatchedRuleSet)
    } else {
        parts = append(parts, "-")
    }
```

**Example:**
- Rule "streaming" matches → hash key includes "streaming"
- Rule "gaming" matches → hash key includes "gaming"
- Same source IP + same ruleset → same outbound

#### 3.3.2 eTLD+1 (Effective Top-Level Domain + 1)

**Purpose:** Route by domain suffix for grouping subdomains.

**Implementation:**
```go
case "etld_plus_one":
    if metadata.Destination.IsFqdn() {
        // Extract eTLD+1 using Public Suffix List
        etld := domain.ExtractETLDPlusOne(metadata.Destination.Fqdn)
        parts = append(parts, etld)
    } else {
        parts = append(parts, "-")
    }
```

**Behavior:**
- `www.example.com` → `example.com`
- `api.example.com` → `example.com`
- `cdn.example.co.uk` → `example.co.uk`
- `192.168.1.1` → `-` (not applicable to IPs)

**Use Case:**
```
Source IP 1.2.3.4 accessing:
- www.google.com → example.com → Outbound A
- youtube.com → youtube.com → Outbound B
- m.youtube.com → youtube.com → Outbound B (same!)
```

**File: `common/domain/etld.go`** (required dependency)

```go
import "golang.org/x/net/publicsuffix"

func ExtractETLDPlusOne(domain string) string {
    // Normalize
    domain = strings.ToLower(domain)
    domain = strings.TrimSuffix(domain, ".")

    // Extract eTLD+1
    etld, err := publicsuffix.EffectiveTLDPlusOne(domain)
    if err != nil {
        return domain // Fallback to original
    }

    return etld
}
```

#### 3.3.3 Matched Ruleset or eTLD (Smart Fallback)

**Purpose:** Priority-based routing with fallback.

**Implementation:**
```go
case "matched_ruleset_or_etld":
    if metadata.MatchedRuleSet != "" {
        // Priority 1: Use ruleset if available
        parts = append(parts, metadata.MatchedRuleSet)
    } else if metadata.Destination.IsFqdn() {
        // Priority 2: Fallback to eTLD+1
        etld := domain.ExtractETLDPlusOne(metadata.Destination.Fqdn)
        parts = append(parts, etld)
    } else {
        parts = append(parts, "-")
    }
```

**Use Case:** Unified hashing for both rule-matched and direct connections.

**Example:**
```
User 1.2.3.4:
- Matches "streaming" rule → hash includes "streaming"
- Direct connection to netflix.com → hash includes "netflix.com"
- Both use consistent routing strategy
```

#### 3.3.4 Destination ASN

**Purpose:** Route by ISP/CDN/cloud provider network.

**Requirements:**
- ASN database (GeoLite2-ASN.mmdb)
- Router ASN reader configured

**Implementation:**
```go
case "dst_asn":
    if lb.asnReader != nil && metadata.Destination.IsValid() && !metadata.Destination.IsFqdn() {
        asn := lb.asnReader.Lookup(metadata.Destination.Addr)
        if asn != 0 {
            parts = append(parts, fmt.Sprintf("AS%d", asn))
        } else {
            parts = append(parts, "-")
        }
    } else {
        parts = append(parts, "-")
    }
```

**ASN Examples:**
- AS16509: Amazon AWS
- AS13335: Cloudflare
- AS15169: Google
- AS8075: Microsoft Azure

**Use Case:**
```
Source IP 1.2.3.4 accessing:
- 13.32.1.1 (AS16509 Amazon) → Outbound A (CDN-optimized)
- 104.16.1.1 (AS13335 Cloudflare) → Outbound B
- 142.250.1.1 (AS15169 Google) → Outbound C
```

**Configuration Required:**
```json
{
    "route": {
        "asn": {
            "path": "/path/to/GeoLite2-ASN.mmdb"
        }
    }
}
```

#### 3.3.5 Destination Geosite

**Purpose:** Route by content category (google, netflix, openai, etc.).

**Requirements:**
- Geosite database (geosite.db)
- Router geosite reader configured

**Implementation:**
```go
case "dst_geosite":
    if lb.geositeReader != nil && metadata.Destination.IsFqdn() {
        code := lb.geositeReader.Lookup(metadata.Destination.Fqdn)
        if code != "" {
            parts = append(parts, fmt.Sprintf("geosite:%s", code))
        } else {
            parts = append(parts, "-")
        }
    } else {
        parts = append(parts, "-")
    }
```

**Geosite Examples:**
- youtube.com → geosite:google
- googlevideo.com → geosite:google
- netflix.com → geosite:netflix
- openai.com → geosite:openai

**Use Case:**
```
Source IP 1.2.3.4 accessing:
- youtube.com → geosite:google → Outbound A
- googlevideo.com → geosite:google → Outbound A (same!)
- netflix.com → geosite:netflix → Outbound B
```

**Configuration Required:**
```json
{
    "route": {
        "geosite": {
            "path": "/path/to/geosite.db"
        }
    }
}
```

### 3.4 Configuration Examples

#### 3.4.1 Ruleset-Based Routing

```json
{
    "type": "loadbalance",
    "tag": "lb-ruleset",
    "primary_outbounds": ["out1", "out2", "out3"],
    "strategy": "consistent_hash",
    "hash": {
        "key_parts": ["src_ip", "matched_ruleset"],
        "virtual_nodes": 100
    }
}
```

Result: Same user + same ruleset → same outbound

#### 3.4.2 Domain Suffix Grouping

```json
{
    "type": "loadbalance",
    "tag": "lb-domain",
    "primary_outbounds": ["out1", "out2", "out3"],
    "strategy": "consistent_hash",
    "hash": {
        "key_parts": ["src_ip", "etld_plus_one"],
        "virtual_nodes": 100
    }
}
```

Result: All subdomains of example.com → same outbound for each user

#### 3.4.3 Smart Fallback Mode

```json
{
    "type": "loadbalance",
    "tag": "lb-smart",
    "primary_outbounds": ["out1", "out2", "out3"],
    "strategy": "consistent_hash",
    "hash": {
        "key_parts": ["src_ip", "matched_ruleset_or_etld"],
        "virtual_nodes": 100
    }
}
```

Result: Uses ruleset if available, otherwise uses domain suffix

#### 3.4.4 CDN Optimization

```json
{
    "type": "loadbalance",
    "tag": "lb-cdn",
    "primary_outbounds": ["out1", "out2", "out3"],
    "strategy": "consistent_hash",
    "hash": {
        "key_parts": ["src_ip", "dst_asn"],
        "virtual_nodes": 100
    }
}
```

Result: All Amazon CDN traffic → same outbound (session affinity)

#### 3.4.5 Service Isolation

```json
{
    "type": "loadbalance",
    "tag": "lb-service",
    "primary_outbounds": ["out1", "out2", "out3"],
    "strategy": "consistent_hash",
    "hash": {
        "key_parts": ["src_ip", "dst_geosite"],
        "virtual_nodes": 100
    }
}
```

Result: All Google services → Outbound A, all Netflix → Outbound B

### 3.5 Migration Steps

1. **Update hash key builder** (`protocol/group/loadbalance.go`)
   - Add new case statements in `buildHashKey()` function
   - Implement eTLD+1 extraction
   - Add ASN lookup logic
   - Add geosite lookup logic

2. **Add domain utility** (`common/domain/etld.go` - NEW FILE if not exists)
   - Import `golang.org/x/net/publicsuffix`
   - Implement `ExtractETLDPlusOne()` function
   - Add domain normalization

3. **Update metadata tracking** (`adapter/inbound.go`)
   - Ensure `MatchedRuleSet` field exists and is populated
   - Verify ASN reader integration
   - Verify geosite reader integration

4. **Documentation**
   - Update hash configuration docs
   - Add use case examples
   - Document database requirements

5. **Testing**
   - Test each key part independently
   - Test combinations
   - Verify hash stability (same key → same outbound)
   - Test ASN/geosite database integration

### 3.6 Testing

```bash
# Test eTLD+1 extraction
go test -v ./common/domain -run TestETLDPlusOne

# Test hash key generation
go test -v ./protocol/group -run TestHashKey

# Test ASN integration
go test -v ./protocol/group -run TestHashKeyASN

# Test geosite integration
go test -v ./protocol/group -run TestHashKeyGeosite

# Test full load balancer
go test -v ./test -run TestLoadBalance
```

---

## 4. TieredLoadBalance Outbound (NEW)

### 4.1 Overview

**Major New Feature:** N-tier load balancer with **real connection latency tracking** (not URL test based).

**Key Innovation:** Measures actual TCP/UDP dial time instead of HTTP probes.

**Architecture:**
- Support unlimited tiers (not just 2)
- Per-tier failure tracking
- Automatic cascade (tier 1 → 2 → 3 → ...)
- Same outbound can appear in multiple tiers with different thresholds

**Benefits:**
- ⚡ **Zero network overhead**: No HTTP test connections
- 🎯 **Real performance**: Tracks actual user-facing latency
- 🔄 **Smart failover**: Automatic tier switching based on performance
- 📊 **Flexible**: N tiers with per-tier top-N selection

### 4.2 Architecture Diagram

```
TieredLoadBalance
├── Latency Tracker (per-outbound, per-tier)
│   ├── Ring buffer: Moving average latency
│   ├── Per-tier failure counters
│   ├── Health determination
│   └── Configurable sampling
│
├── Tier Manager
│   ├── Active tier determination
│   ├── Cascade logic (1 → 2 → 3)
│   ├── Recovery with hold time
│   └── Hysteresis prevention
│
├── Candidate Pools (per-tier)
│   ├── Top-N selection by latency
│   ├── Hash rings (optional)
│   └── Atomic snapshot updates
│
└── URLTest Support (optional, Clash API)
    ├── Periodic HTTP health checks
    ├── History storage
    └── Does NOT affect tier selection
```

### 4.3 Configuration Schema

**File: `option/group.go`**

```go
type TieredLoadBalanceOutboundOptions struct {
    Tiers                     []LoadBalanceTierOptions  `json:"tiers"`
    Strategy                  string                    `json:"strategy,omitempty"`
    Hash                      *LoadBalanceHashOptions   `json:"hash,omitempty"`
    LatencyMonitoring         *LatencyMonitoringOptions `json:"latency_monitoring,omitempty"`
    EmptyPoolAction           string                    `json:"empty_pool_action,omitempty"`
    InterruptExistConnections bool                      `json:"interrupt_exist_connections,omitempty"`

    // URLTest support (optional, for Clash API)
    URL         string             `json:"url,omitempty"`
    Interval    badoption.Duration `json:"interval,omitempty"`
    Timeout     badoption.Duration `json:"timeout,omitempty"`
    IdleTimeout badoption.Duration `json:"idle_timeout,omitempty"`
}

type LoadBalanceTierOptions struct {
    Level      int                `json:"level"`
    Outbounds  []string           `json:"outbounds"`
    TopN       int                `json:"top_n"`
    Strategy   string             `json:"strategy,omitempty"`  // Can override global
    MaxLatency badoption.Duration `json:"max_latency"`
}

type LatencyMonitoringOptions struct {
    FailureThreshold   uint32             `json:"failure_threshold,omitempty"`
    RecoveryThreshold  uint32             `json:"recovery_threshold,omitempty"`
    SamplingRate       int                `json:"sampling_rate,omitempty"`
    HistorySize        int                `json:"history_size,omitempty"`
    FallbackHoldTime   badoption.Duration `json:"fallback_hold_time,omitempty"`
    MeasurementTimeout badoption.Duration `json:"measurement_timeout,omitempty"`
}
```

### 4.4 Core Implementation

#### 4.4.1 Latency Tracker

**File: `protocol/group/latency_tracker.go`** (NEW, 300 lines)

```go
type LatencyTracker struct {
    mu        sync.RWMutex
    outbounds map[string]*OutboundStats

    // Configuration
    failureThreshold  uint32  // Consecutive failures to mark unhealthy
    recoveryThreshold uint32  // Consecutive successes to mark healthy
    historySize       int     // Latency history window size
    samplingRate      int     // Sample 1 in N connections
    samplingCounter   atomic.Uint64
}

type OutboundStats struct {
    tag string
    mu  sync.RWMutex

    // Latency history (ring buffer for moving average)
    latencyHistory []time.Duration
    historyIndex   int
    historyFull    bool

    // Per-tier failure tracking
    tierFailures map[int]*TierFailureState
}

type TierFailureState struct {
    consecutiveFailures  uint32
    consecutiveSuccesses uint32
    maxLatency           time.Duration
}

// Core method: Record actual connection latency
func (lt *LatencyTracker) RecordLatency(
    tag string,
    tierLevel int,
    duration time.Duration,
    success bool,
) {
    stats := lt.getOrCreateStats(tag)
    stats.mu.Lock()
    defer stats.mu.Unlock()

    // Update history on success
    if success {
        stats.addLatencyLocked(duration)
    }

    // Check per-tier threshold
    tierState := stats.tierFailures[tierLevel]
    if tierState == nil {
        return
    }

    if !success || duration > tierState.maxLatency {
        // Failed or too slow for this tier
        tierState.consecutiveFailures++
        tierState.consecutiveSuccesses = 0
    } else {
        // Success within threshold
        tierState.consecutiveFailures = 0
        tierState.consecutiveSuccesses++
    }
}

// Check health for specific tier
func (lt *LatencyTracker) IsHealthyForTier(tag string, tierLevel int) bool {
    stats := lt.outbounds[tag]
    if stats == nil {
        return true // Unknown = optimistic (cold start)
    }

    tierState := stats.tierFailures[tierLevel]
    if tierState == nil {
        return true
    }

    return tierState.consecutiveFailures < lt.failureThreshold
}

// Get moving average latency
func (lt *LatencyTracker) GetAverageLatency(tag string) time.Duration {
    stats := lt.outbounds[tag]
    if stats == nil || len(stats.latencyHistory) == 0 {
        return 0
    }

    var sum time.Duration
    for _, latency := range stats.latencyHistory {
        sum += latency
    }

    return sum / time.Duration(len(stats.latencyHistory))
}

// Sampling decision
func (lt *LatencyTracker) ShouldSample() bool {
    if lt.samplingRate == 1 {
        return true
    }
    counter := lt.samplingCounter.Add(1)
    return (counter % uint64(lt.samplingRate)) == 0
}
```

#### 4.4.2 Main Load Balancer

**File: `protocol/group/tiered_loadbalance.go`** (NEW, 1156 lines)

```go
type TieredLoadBalance struct {
    outbound.Adapter

    ctx      context.Context
    router   adapter.Router
    outbound adapter.OutboundManager
    logger   log.ContextLogger

    // Configuration
    tiers           []TierConfig
    globalStrategy  string
    emptyPoolAction string

    // Runtime components
    latencyTracker  *LatencyTracker
    tierState       atomic.Value // *TierStateSnapshot
    candidatePools  atomic.Value // *CandidatePoolSnapshot

    // Hash configuration
    hashKeyParts     []string
    hashVirtualNodes int
    hashOnEmptyKey   string
    hashKeySalt      string

    // Hysteresis
    fallbackHoldTime time.Duration

    // Resources
    interruptGroup               *interrupt.Group
    interruptExternalConnections bool
    asnReader                    adapter.ASNReader
    geositeReader                adapter.GeositeReader

    // URLTest support (optional)
    link         string
    interval     time.Duration
    timeout      time.Duration
    idleTimeout  time.Duration
    history      adapter.URLTestHistoryStorage
    pauseManager pause.Manager
    ticker       *time.Ticker
    checking     atomic.Bool
}

type TierConfig struct {
    level      int
    tags       []string
    topN       int
    strategy   string // Can override global strategy
    maxLatency time.Duration
}

type TierStateSnapshot struct {
    activeTierLevel int
    tierActivatedAt time.Time
    previousTier    int
}

type CandidatePoolSnapshot struct {
    tierCandidates map[int][]adapter.Outbound
    tierHashRings  map[int]*consistentHashRing
}
```

**Key Methods:**

```go
// 1. Dial with latency measurement
func (lb *TieredLoadBalance) DialContext(ctx, network, destination) (net.Conn, error) {
    metadata := adapter.ContextFrom(ctx)

    // Select outbound from active tier
    outbound, tierLevel, err := lb.selectOutbound(network, metadata)
    if err != nil {
        return nil, err
    }

    // Measure actual dial time
    start := time.Now()
    conn, err := outbound.DialContext(ctx, network, destination)
    duration := time.Since(start)

    // Record latency asynchronously (if sampled)
    if lb.latencyTracker.ShouldSample() {
        go func() {
            lb.latencyTracker.RecordLatency(
                outbound.Tag(),
                tierLevel,
                duration,
                err == nil,
            )
            lb.updateCandidates()
        }()
    }

    if err != nil {
        return nil, err
    }

    if lb.interruptGroup != nil {
        return lb.interruptGroup.NewConn(conn,
            interrupt.IsExternalConnectionFromContext(ctx)), nil
    }

    return conn, nil
}

// 2. Select outbound from tiers (cascade)
func (lb *TieredLoadBalance) selectOutbound(network, metadata) (adapter.Outbound, int, error) {
    pools := lb.candidatePools.Load().(*CandidatePoolSnapshot)
    tierState := lb.tierState.Load().(*TierStateSnapshot)

    activeTier := tierState.activeTierLevel

    // Try tiers in order starting from active tier
    for tierLevel := activeTier; tierLevel <= len(lb.tiers); tierLevel++ {
        candidates, exists := pools.tierCandidates[tierLevel]
        if !exists || len(candidates) == 0 {
            continue
        }

        // Filter by network support
        networkCandidates := filterByNetwork(candidates, network)
        if len(networkCandidates) == 0 {
            continue
        }

        // Select from this tier using strategy
        tier := lb.tiers[tierLevel-1]
        var selected adapter.Outbound

        if tier.strategy == strategyConsistentHash {
            selected = lb.selectConsistentHash(pools, tierLevel, networkCandidates, metadata)
        } else {
            selected = networkCandidates[rand.Intn(len(networkCandidates))]
        }

        if selected != nil {
            return selected, tierLevel, nil
        }
    }

    // All tiers exhausted
    if lb.emptyPoolAction == emptyPoolActionFallbackAll {
        return lb.selectFallbackOutbound(network)
    }

    return nil, 0, E.New("no healthy candidates in any tier")
}

// 3. Update candidate pools
func (lb *TieredLoadBalance) updateCandidates() {
    newPools := &CandidatePoolSnapshot{
        tierCandidates: make(map[int][]adapter.Outbound),
        tierHashRings:  make(map[int]*consistentHashRing),
    }

    // Build candidate pool for each tier
    for _, tier := range lb.tiers {
        candidates := lb.selectTopNForTier(tier)
        newPools.tierCandidates[tier.level] = candidates

        // Log candidates
        lb.logCandidates(tier.level, candidates)

        // Build hash ring if needed
        if tier.strategy == strategyConsistentHash && len(candidates) > 0 {
            newPools.tierHashRings[tier.level] = lb.buildHashRing(candidates)
        }
    }

    // Check tier transition
    lb.checkTierTransition(newPools)

    // Atomically update pools
    lb.candidatePools.Swap(newPools)
}

// 4. Select top-N for tier
func (lb *TieredLoadBalance) selectTopNForTier(tier TierConfig) []adapter.Outbound {
    infos := []tierLatencyInfo{}

    for _, tag := range tier.tags {
        healthy := lb.latencyTracker.IsHealthyForTier(tag, tier.level)
        if !healthy {
            continue
        }

        latency := lb.latencyTracker.GetAverageLatency(tag)
        infos = append(infos, tierLatencyInfo{
            tag:     tag,
            latency: latency,
            healthy: true,
        })
    }

    if len(infos) == 0 {
        return nil
    }

    // Sort by latency (0 = unknown, sorts first for cold start)
    sort.Slice(infos, func(i, j int) bool {
        if infos[i].latency == 0 && infos[j].latency != 0 {
            return true
        }
        if infos[i].latency != 0 && infos[j].latency == 0 {
            return false
        }
        return infos[i].latency < infos[j].latency
    })

    // Select top-N
    topN := tier.topN
    if topN > len(infos) {
        topN = len(infos)
    }

    candidates := []adapter.Outbound{}
    for i := 0; i < topN; i++ {
        outbound, loaded := lb.outbound.Outbound(infos[i].tag)
        if loaded {
            candidates = append(candidates, outbound)
        }
    }

    return candidates
}

// 5. Check tier transition
func (lb *TieredLoadBalance) checkTierTransition(pools *CandidatePoolSnapshot) {
    currentState := lb.tierState.Load().(*TierStateSnapshot)
    activeTier := currentState.activeTierLevel

    // Try to recover to lower (better) tier
    for tierLevel := 1; tierLevel < activeTier; tierLevel++ {
        candidates, exists := pools.tierCandidates[tierLevel]
        if exists && len(candidates) > 0 {
            // Check hold time
            if time.Since(currentState.tierActivatedAt) >= lb.fallbackHoldTime {
                lb.logger.Warn(
                    "tier recovery: tier ", activeTier, " -> tier ", tierLevel,
                )

                newState := &TierStateSnapshot{
                    activeTierLevel: tierLevel,
                    tierActivatedAt: time.Now(),
                    previousTier:    activeTier,
                }
                lb.tierState.Store(newState)
                return
            }
        }
    }

    // Try to fallback to higher (worse) tier
    candidates, exists := pools.tierCandidates[activeTier]
    if !exists || len(candidates) == 0 {
        for tierLevel := activeTier + 1; tierLevel <= len(lb.tiers); tierLevel++ {
            candidates, exists := pools.tierCandidates[tierLevel]
            if exists && len(candidates) > 0 {
                lb.logger.Warn(
                    "tier fallback: tier ", activeTier, " -> tier ", tierLevel,
                )

                newState := &TierStateSnapshot{
                    activeTierLevel: tierLevel,
                    tierActivatedAt: time.Now(),
                    previousTier:    activeTier,
                }
                lb.tierState.Store(newState)
                return
            }
        }
    }
}

// 6. URLTest support (optional, for Clash API)
func (lb *TieredLoadBalance) URLTest(ctx) (map[string]uint16, error) {
    // Perform HTTP health checks
    // Store results in history
    // Does NOT affect tier selection (real latency is primary)
    // ... implementation similar to LoadBalance ...
}
```

### 4.5 Configuration Example

```json
{
    "type": "tiered_loadbalance",
    "tag": "lb-auto",

    "tiers": [
        {
            "level": 1,
            "outbounds": ["fast1", "fast2", "fast3"],
            "top_n": 2,
            "max_latency": "100ms"
        },
        {
            "level": 2,
            "outbounds": ["fast1", "medium1", "medium2"],
            "top_n": 2,
            "strategy": "random",
            "max_latency": "200ms"
        },
        {
            "level": 3,
            "outbounds": ["backup1", "backup2"],
            "top_n": 1,
            "max_latency": "500ms"
        }
    ],

    "strategy": "consistent_hash",
    "hash": {
        "key_parts": ["src_ip", "dst_geosite"],
        "virtual_nodes": 100
    },

    "latency_monitoring": {
        "failure_threshold": 3,
        "recovery_threshold": 2,
        "sampling_rate": 1,
        "history_size": 10,
        "fallback_hold_time": "30s"
    },

    "empty_pool_action": "fallback_all",
    "interrupt_exist_connections": true,

    "url": "https://www.gstatic.com/generate_204",
    "interval": "3m",
    "timeout": "5s",
    "idle_timeout": "30m"
}
```

### 4.6 Key Behaviors

#### 4.6.1 Per-Tier Failure Tracking

**Scenario:** `fast1` appears in both tier 1 and tier 2

```
Tier 1: max_latency=100ms
├── fast1: 30ms, 40ms, 110ms, 120ms, 115ms
└── Status: 3 consecutive failures for tier 1 → UNHEALTHY

Tier 2: max_latency=200ms
├── fast1: same latencies as above
└── Status: All within 200ms → HEALTHY

Result:
- fast1 excluded from tier 1 candidates
- fast1 still serves tier 2
- Independent failure counters per tier
```

#### 4.6.2 Tier Cascade

```
Initial State: Tier 1 active

Tier 1 pool becomes empty:
├── Immediately cascade to tier 2
└── No hold time for fallback

Tier 1 recovers (has candidates):
├── Wait for fallback_hold_time (default 30s)
├── Prevents rapid oscillation
└── Then switch back to tier 1
```

#### 4.6.3 Cold Start

```
No latency data available:
├── All outbounds assumed healthy
├── Select all tier 1 outbounds (up to top_n)
├── Latency = 0 (sorts first)
└── Start collecting real latency data

After first connection:
├── Record actual dial time
├── Update moving average
└── Next selection uses real data
```

### 4.7 Performance Characteristics

**Memory Overhead per outbound:**
- Latency history: 10 entries × 8 bytes = 80 bytes
- Per-tier state: ~50 bytes × N tiers
- **Total: ~200 bytes per outbound**

For 100 outbounds across 4 tiers: **~20 KB** (negligible)

**CPU Overhead per connection:**
- 2× `time.Now()` calls: ~200ns
- 1× atomic load: ~10ns
- 1× goroutine spawn (async): ~1μs amortized
- **Total: ~1.2μs per connection**

For 1000 connections/sec: **0.12% CPU** (negligible)

**Latency Impact:**
- Zero network overhead (no HTTP probes)
- Measurement is local `time.Now()` only

### 4.8 Migration Steps

1. **Add configuration schema** (`option/group.go`)
   - `TieredLoadBalanceOutboundOptions`
   - `LoadBalanceTierOptions`
   - `LatencyMonitoringOptions`

2. **Add type constant** (`constant/proxy.go`)
   - `TypeTieredLoadBalance = "tiered_loadbalance"`

3. **Implement latency tracker** (`protocol/group/latency_tracker.go` - NEW FILE)
   - `LatencyTracker` type
   - `OutboundStats` type
   - `TierFailureState` type
   - Core methods: RecordLatency, IsHealthyForTier, GetAverageLatency

4. **Implement main load balancer** (`protocol/group/tiered_loadbalance.go` - NEW FILE)
   - `TieredLoadBalance` type
   - DialContext/ListenPacket with latency measurement
   - selectOutbound with tier cascade
   - updateCandidates with per-tier pools
   - checkTierTransition with hysteresis
   - URLTest support (optional)

5. **Register outbound type** (`include/registry.go`)
   - `group.RegisterTieredLoadBalance(registry)`

6. **Add tests** (`protocol/group/tiered_loadbalance_test.go` - NEW FILE)
   - Latency tracker tests (10 test cases)
   - Configuration validation
   - Tier selection logic
   - Race detector validation

7. **Documentation**
   - Configuration guide
   - Architecture overview
   - Use case examples
   - Migration from LoadBalance

### 4.9 Testing

**File: `protocol/group/tiered_loadbalance_test.go`** (186 lines)

```go
func TestLatencyTracker(t *testing.T) {
    t.Run("RecordLatency", func(t *testing.T) {
        // Test basic latency recording and threshold detection
    })

    t.Run("PerTierFailureTracking", func(t *testing.T) {
        // Test same outbound in multiple tiers
        // Verify independent failure counters
    })

    t.Run("AverageLatency", func(t *testing.T) {
        // Test moving average calculation
        // Test ring buffer wrap-around
    })

    t.Run("SamplingRate", func(t *testing.T) {
        // Test sampling 1 in N connections
    })

    t.Run("Recovery", func(t *testing.T) {
        // Test recovery from unhealthy state
    })

    t.Run("UnknownOutbound", func(t *testing.T) {
        // Test optimistic cold start
    })

    t.Run("FailureConnection", func(t *testing.T) {
        // Test connection failure handling
    })

    t.Run("MultipleOutbounds", func(t *testing.T) {
        // Test concurrent tracking
    })

    t.Run("TierStats", func(t *testing.T) {
        // Test statistics retrieval
    })

    t.Run("Reset", func(t *testing.T) {
        // Test state reset
    })
}
```

**Run tests:**
```bash
go test -v ./protocol/group -run TestLatencyTracker
go test -race ./protocol/group
```

---

## 5. Sniff Override Destination

### 5.1 Overview

**Legacy Feature:** Allows sniffing results to override the destination address.

**Status:** Already exists but marked as deprecated. Being kept for backward compatibility.

**Location:** Inbound options (not route rule action).

### 5.2 Configuration Schema

**File: `option/inbound.go`**

```go
// Deprecated: Use rule action instead
type InboundOptions struct {
    SniffEnabled              bool               `json:"sniff,omitempty"`
    SniffOverrideDestination  bool               `json:"sniff_override_destination,omitempty"`
    SniffTimeout              badoption.Duration `json:"sniff_timeout,omitempty"`
    DomainStrategy            DomainStrategy     `json:"domain_strategy,omitempty"`
    UDPDisableDomainUnmapping bool               `json:"udp_disable_domain_unmapping,omitempty"`
    Detour                    string             `json:"detour,omitempty"`
}
```

### 5.3 Implementation

**File: `route/route.go`**

```go
// Legacy sniff action handling
if metadata.InboundOptions.SniffEnabled {
    newBuffer, newPacketBuffers, newErr := r.actionSniff(
        ctx,
        metadata,
        &R.RuleActionSniff{
            OverrideDestination: metadata.InboundOptions.SniffOverrideDestination,
            Timeout:             time.Duration(metadata.InboundOptions.SniffTimeout),
        },
        inputConn,
        inputPacketConn,
        nil,
        nil,
    )
    // ... handle result ...
}
```

### 5.4 Behavior

**When `sniff_override_destination: true`:**
- Connection comes in for `example.com:443`
- Sniffer detects TLS SNI: `www.real-destination.com`
- Destination is overridden to `www.real-destination.com:443`
- Routing rules see the real destination

**When `sniff_override_destination: false`:**
- Original destination preserved
- Sniff result stored but doesn't change routing

### 5.5 Configuration Example

```json
{
    "inbounds": [
        {
            "type": "mixed",
            "tag": "mixed-in",
            "listen": "127.0.0.1",
            "listen_port": 1080,

            "sniff": true,
            "sniff_override_destination": true,
            "sniff_timeout": "300ms"
        }
    ]
}
```

### 5.6 Migration Notes

**Important:** This is a **legacy feature** marked as deprecated.

**Modern approach:** Use route rule actions instead:

```json
{
    "route": {
        "rules": [
            {
                "action": "sniff",
                "override_destination": true,
                "timeout": "300ms"
            }
        ]
    }
}
```

**Migration Strategy:**
1. **Keep the legacy option** for backward compatibility
2. **Mark as deprecated** in documentation
3. **Recommend** rule action approach for new configs
4. **Eventually remove** in future major version

### 5.7 Testing

```bash
# Test sniff override
# 1. Configure inbound with sniff_override_destination: true
# 2. Connect to proxy
# 3. Request https://example.com
# 4. Server sees SNI for different domain
# 5. Verify routing uses SNI domain, not original request
```

---

## Implementation Checklist

### Phase 1: Low Complexity (Week 1)

- [ ] **AnyTLS Enhancements**
  - [ ] Add config options
  - [ ] Pass through to sing-anytls
  - [ ] Update documentation
  - [ ] Test heartbeat
  - [ ] Test rate limiting

- [ ] **Sniff Override**
  - [ ] Verify existing implementation
  - [ ] Mark as deprecated
  - [ ] Document modern alternative
  - [ ] Test legacy behavior

### Phase 2: Medium Complexity (Week 2)

- [ ] **LoadBalance Hash Features**
  - [ ] Implement matched_ruleset
  - [ ] Implement etld_plus_one
  - [ ] Implement matched_ruleset_or_etld
  - [ ] Implement dst_asn
  - [ ] Implement dst_geosite
  - [ ] Add domain utility
  - [ ] Update documentation
  - [ ] Test each key part
  - [ ] Test combinations

### Phase 3: High Complexity (Week 3-4)

- [ ] **VLESS Connection Pool**
  - [ ] Add config schema
  - [ ] Implement ConnectionPool
  - [ ] Integrate with outbound
  - [ ] Add pooledVLESSConn wrapper
  - [ ] Write 18 test cases
  - [ ] Race detector validation
  - [ ] Performance benchmarks
  - [ ] Documentation

- [ ] **TieredLoadBalance**
  - [ ] Add config schema
  - [ ] Implement LatencyTracker
  - [ ] Implement TieredLoadBalance
  - [ ] Add tier selection logic
  - [ ] Add tier transition logic
  - [ ] Add URLTest support
  - [ ] Register outbound type
  - [ ] Write comprehensive tests
  - [ ] Documentation

### Phase 4: Integration & Testing (Week 5)

- [ ] **Integration Testing**
  - [ ] Test all features together
  - [ ] Cross-feature compatibility
  - [ ] Performance testing
  - [ ] Memory profiling
  - [ ] Race condition testing

- [ ] **Documentation**
  - [ ] User guide
  - [ ] Migration guide
  - [ ] Examples
  - [ ] Troubleshooting

- [ ] **Release Preparation**
  - [ ] Changelog
  - [ ] Version bump
  - [ ] Pre-built binaries
  - [ ] Announcement

---

## Testing Strategy

### Unit Tests

```bash
# AnyTLS
go test -v ./protocol/anytls

# VLESS Pool
go test -v ./protocol/vless -run TestConnectionPool
go test -race ./protocol/vless

# LoadBalance
go test -v ./protocol/group -run TestHashKey
go test -v ./protocol/group -run TestLoadBalance

# TieredLoadBalance
go test -v ./protocol/group -run TestLatencyTracker
go test -race ./protocol/group
```

### Integration Tests

```bash
# Full stack test
go test -v ./test -run TestLoadBalance
go test -v ./test -run TestTieredLoadBalance
go test -v ./test -run TestVLESSPool
```

### Performance Tests

```bash
# Benchmarks
go test -bench=. ./protocol/vless
go test -bench=. ./protocol/group

# Memory profiling
go test -memprofile=mem.prof ./protocol/vless
go tool pprof mem.prof

# CPU profiling
go test -cpuprofile=cpu.prof ./protocol/group
go tool pprof cpu.prof
```

### Manual Testing

1. **VLESS Pool:**
   - Monitor connection creation
   - Verify pool size maintenance
   - Test idle timeout
   - Test age rotation
   - Test heartbeat detection

2. **TieredLoadBalance:**
   - Verify tier cascade
   - Check tier recovery
   - Monitor latency tracking
   - Test per-tier health

3. **LoadBalance Hash:**
   - Test each key part
   - Verify consistent hashing
   - Check ASN/geosite integration

---

## Code References

### Key Files

| File | Lines | Purpose |
|------|-------|---------|
| `option/anytls.go` | 32 | AnyTLS config |
| `option/vless.go` | 52 | VLESS config |
| `option/group.go` | 126 | LoadBalance config |
| `protocol/vless/pool.go` | 441 | VLESS pool impl |
| `protocol/group/latency_tracker.go` | 300 | Latency tracking |
| `protocol/group/tiered_loadbalance.go` | 1156 | Tiered LB impl |
| `protocol/group/loadbalance.go` | ~1324 | LoadBalance impl |

### Git Commits

```bash
# AnyTLS
git show 51f23e86  # heartbeat
git show 5c88b6b1  # create_rate

# VLESS Pool
git show 38b04fe0  # implementation

# LoadBalance
git show 4622c265  # hash features

# TieredLoadBalance
git show 16e18938  # initial impl
git show bbcbdbd5  # URLTest enrichment
```

---

## Notes for Successor AI

### Critical Considerations

1. **Backward Compatibility:**
   - All features are additive (no breaking changes)
   - Optional configurations (disabled by default)
   - Legacy features marked deprecated but functional

2. **Performance:**
   - VLESS pool: <1% CPU overhead, ~200 bytes per connection
   - TieredLoadBalance: <1.2μs per connection, ~200 bytes per outbound
   - Both designed for high-traffic scenarios

3. **Thread Safety:**
   - All shared state uses atomic operations or mutexes
   - Test with `-race` flag
   - No data races allowed

4. **Testing:**
   - Comprehensive unit tests required
   - Race detector validation mandatory
   - Integration tests with real traffic

5. **Documentation:**
   - User-facing configuration guide
   - Internal architecture docs
   - Code comments for complex logic

### Common Pitfalls

1. **VLESS Pool:**
   - Don't forget TCP Fast Open validation
   - Pool cleanup must respect min_idle_session
   - Heartbeat loop must handle closed connections
   - Network change requires pool reset

2. **TieredLoadBalance:**
   - Per-tier failure tracking is independent
   - Cold start must be optimistic (no data = healthy)
   - Tier transitions need hysteresis (hold time)
   - Sampling reduces overhead but delays convergence

3. **LoadBalance Hash:**
   - eTLD+1 requires Public Suffix List
   - ASN/geosite require database integration
   - Hash key must be deterministic
   - Virtual nodes affect distribution

### Questions to Ask User

Before implementing, clarify:

1. **Priority:** Which features are most critical?
2. **Timeline:** What's the deadline for each phase?
3. **Testing:** What level of test coverage is required?
4. **Documentation:** English only or multilingual?
5. **Compatibility:** Any version-specific considerations for 1.13.x?

### Success Criteria

Feature is complete when:

- [ ] Code compiles without warnings
- [ ] All tests pass (unit + integration)
- [ ] Race detector shows no issues
- [ ] Performance benchmarks meet targets
- [ ] Documentation is complete
- [ ] User can configure and use feature
- [ ] Feature works with other components

---

## Conclusion

This document provides a complete blueprint for migrating 5 major features from sing-box 1.12.14.x to 1.13.x:

1. ✅ **AnyTLS Enhancements** - Simple pass-through options
2. ✅ **VLESS Connection Pool** - Complex but well-architected
3. ✅ **LoadBalance Hash Features** - Moderate complexity, high value
4. ✅ **TieredLoadBalance** - Most complex, most innovative
5. ✅ **Sniff Override** - Legacy, simple, deprecated

**Estimated Total Effort:** 4-5 weeks for experienced developer

**Recommended Approach:**
1. Start with AnyTLS and Sniff (easy wins)
2. Implement LoadBalance hash features (medium complexity)
3. Tackle VLESS Pool (high complexity, critical path)
4. Finally implement TieredLoadBalance (most complex)

**Success Metrics:**
- Zero breaking changes
- <1% performance overhead
- 100% test coverage on new code
- Full backward compatibility
- Production-ready quality

Good luck with the implementation! 🚀

---

**Document Metadata:**
- Created: 2026-01-26
- Version: 1.0
- Author: Claude Sonnet 4.5
- Target: sing-box 1.13.x
- Source: sing-box 1.12.14.x
