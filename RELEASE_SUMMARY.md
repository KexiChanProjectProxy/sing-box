# Release v1.12.14.9 - Summary

## Status

### ✅ Completed Tasks

1. **Version Bumped** ✅
   - Updated `constant/version.go` from 1.12.14.8 → 1.12.14.9
   - Updated `build-releases.sh` version

2. **Release Notes Created** ✅
   - Created `RELEASE_NOTES_v1.12.14.9.md` with comprehensive feature documentation
   - Created `VLESS_CONNECTION_POOL_IMPLEMENTATION.md` with technical details

3. **Code Committed Locally** ✅
   - Commit: `38b04fe0 feat: Add VLESS connection pool for improved latency and reliability`
   - 9 files changed, 1855 insertions(+), 10 deletions(-)
   - Files added:
     - `protocol/vless/pool.go` (431 lines)
     - `protocol/vless/pool_test.go` (656 lines)
     - `protocol/vless/integration_test.go` (214 lines)
     - Documentation files

4. **Release Binaries Built** ✅
   - All 6 platform binaries built successfully:
     - ✅ sing-box-linux-amd64 (56M)
     - ✅ sing-box-linux-arm64 (53M)
     - ✅ sing-box-windows-amd64.exe (56M)
     - ✅ sing-box-windows-arm64.exe (52M)
     - ✅ sing-box-darwin-amd64 (56M)
     - ✅ sing-box-darwin-arm64 (52M)

### ⚠️ Pending Tasks (Requires Authentication)

5. **Push to GitHub** ⚠️
   - **Issue**: GitHub SSH authentication failed
   - **Error**: `Connection closed by 20.205.243.166 port 22`
   - **Status**: Code committed locally, ready to push
   - **Action Required**: Set up GitHub authentication

6. **Create GitHub Release** ⚠️
   - **Issue**: gh CLI authentication invalid
   - **Error**: `The token in /home/kexi/.config/gh/hosts.yml is invalid`
   - **Status**: Release script ready
   - **Action Required**: Run `gh auth login -h github.com`

## Next Steps

### Option 1: Automatic (Recommended)
Run the prepared script after authentication:

```bash
# 1. Authenticate with GitHub CLI
gh auth login -h github.com

# 2. Run the complete release script
./complete-release.sh
```

### Option 2: Manual Steps
```bash
# 1. Authenticate
gh auth login -h github.com

# 2. Push code
git push origin master

# 3. Create and push tag
git tag v1.12.14.9
git push origin v1.12.14.9

# 4. Create GitHub release
gh release create v1.12.14.9 \
  --title "sing-box v1.12.14.9 - VLESS Connection Pool" \
  --notes-file RELEASE_NOTES_v1.12.14.9.md \
  releases/sing-box-linux-amd64 \
  releases/sing-box-linux-arm64 \
  releases/sing-box-windows-amd64.exe \
  releases/sing-box-windows-arm64.exe \
  releases/sing-box-darwin-amd64 \
  releases/sing-box-darwin-arm64
```

## What Was Implemented

### VLESS Connection Pool Feature
Complete implementation of AnyTLS-style connection pool for VLESS protocol:

**Features:**
- Pre-connections for reduced latency
- Rate limiting to prevent connection storms
- Idle management with configurable cleanup
- Age-based rotation with jitter
- TCP-level keepalive
- Network interface change handling

**Testing:**
- 18 comprehensive tests
- ✅ All tests pass with race detector
- ✅ No data races
- ✅ No goroutine leaks

**Compatibility:**
- ✅ Non-breaking (disabled by default)
- ✅ Works with multiplex
- ✅ Works with all transports
- ❌ TCP Fast Open validation added

## Files Ready for Release

### Source Code
- `constant/version.go` - Version 1.12.14.9
- `option/vless.go` - Connection pool options
- `protocol/vless/pool.go` - Core implementation
- `protocol/vless/pool_test.go` - Unit tests
- `protocol/vless/integration_test.go` - Integration tests
- `protocol/vless/outbound.go` - Integration

### Documentation
- `RELEASE_NOTES_v1.12.14.9.md` - User-facing release notes
- `VLESS_CONNECTION_POOL_IMPLEMENTATION.md` - Technical documentation

### Binaries (releases/)
All binaries built with version 1.12.14.9 and full feature set:
```
-rwxrwxr-x 56M sing-box-linux-amd64
-rwxrwxr-x 53M sing-box-linux-arm64
-rwxrwxr-x 56M sing-box-windows-amd64.exe
-rwxrwxr-x 52M sing-box-windows-arm64.exe
-rwxrwxr-x 56M sing-box-darwin-amd64
-rwxrwxr-x 52M sing-box-darwin-arm64
```

### Scripts
- `complete-release.sh` - Automated release completion script
- `build-releases.sh` - Updated with v1.12.14.9

## Authentication Setup

### GitHub CLI (gh)
```bash
gh auth login -h github.com
```
Choose:
- Account: github.com
- Protocol: HTTPS or SSH
- Authenticate via: Browser or Token

### SSH (Alternative)
If using SSH, ensure your SSH key is added to GitHub:
```bash
# Generate key if needed
ssh-keygen -t ed25519 -C "your_email@example.com"

# Add to ssh-agent
eval "$(ssh-agent -s)"
ssh-add ~/.ssh/id_ed25519

# Add public key to GitHub
cat ~/.ssh/id_ed25519.pub
# Copy and paste to: https://github.com/settings/keys
```

## Release URL (After Completion)
https://github.com/KexiChanProjectProxy/sing-box/releases/tag/v1.12.14.9

## Verification After Release

1. **Check Release Page**
   - Verify all 6 binaries are uploaded
   - Verify release notes are displayed correctly

2. **Test Download**
   ```bash
   # Download and verify a binary
   gh release download v1.12.14.9 --pattern "sing-box-linux-amd64"
   ./sing-box-linux-amd64 version
   # Should show: 1.12.14.9
   ```

3. **Test Configuration**
   - Test VLESS connection pool with example config
   - Verify backward compatibility (pool disabled by default)

---

**Prepared by**: Claude Code
**Date**: January 24, 2026
**Commit**: 38b04fe0
