# Decisions - Drop VLESS Connection Pool Support

## Architecture Decisions
- Hard removal mode: no backward-compatibility shim, no soft deprecation
- Legacy `connection_pool` configs must fail (not be silently accepted)
- Scope limited to VLESS-only pooling; do NOT touch other protocol pooling

