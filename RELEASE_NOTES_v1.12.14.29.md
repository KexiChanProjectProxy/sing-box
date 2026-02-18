# Release Notes v1.12.14.29

## New Features

### LoadBalance: Configurable Tolerance for Top-N Selection

Added a `tolerance` field to the LoadBalance outbound that stabilizes the Top-N candidate pool across health check cycles. This feature prevents unnecessary candidate pool churn when nodes have similar latencies, which is particularly beneficial for `consistent_hash` strategy where it reduces hash ring rebuilds and maintains sticky sessions.

**Configuration:**

```json
{
  "type": "loadbalance",
  "tag": "lb",
  "primary_outbounds": ["proxy-a", "proxy-b", "proxy-c"],
  "top_n": {
    "primary": 3
  },
  "tolerance": 50,
  "strategy": "consistent_hash",
  "hash": {
    "key_parts": ["src_ip"]
  }
}
```

**Behavior:**

- When `tolerance: 0` (default): Pure Top-N selection (backward compatible)
- When `tolerance > 0`: Previous Top-N candidates within `tolerance` ms of the N-th ranked node remain eligible, but the best N nodes from the expanded pool are still selected

**Example:** With `top_n.primary: 3` and `tolerance: 50`:
- Pure Top-N selection would pick nodes at 48ms, 50ms, 51ms
- If a previous candidate was at 55ms (within 50ms of 51ms + 50ms tolerance), it stays eligible
- The final selection still picks the best 3 from the eligible set

This reduces hash ring rebuilds when nodes have similar latencies, preventing sticky session disruptions.

**Edge Cases Handled:**

- First health check (no previous snapshot): Pure Top-N
- Previous candidate failed health check: Not retained
- Previous candidate exceeds tolerance: Not eligible
- N ≥ number of success nodes: All selected (tolerance has no effect)

## Technical Details

**Implementation:**

- Added `Tolerance uint16` field to `LoadBalanceOutboundOptions` in `option/group.go`
- Modified `selectTopN()` to accept previous candidate tags and apply tolerance logic
- Updated `updateCandidates()` to extract and pass previous candidate tags
- Added comprehensive unit tests covering all edge cases

**Files Modified:**

- `option/group.go`: Added config field
- `protocol/group/loadbalance.go`: Core implementation
- `protocol/group/loadbalance_test.go`: 6 new test cases
- `docs/configuration/outbound/loadbalance.md`: Documentation
- `docs/configuration/outbound/loadbalance.zh.md`: Chinese documentation
