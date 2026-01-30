# Documentation Update Summary - v1.12.14.9

## Overview
Comprehensive documentation has been added for the VLESS connection pool feature in both English and Chinese.

## Files Updated

### 1. English Documentation
**File**: `docs/configuration/outbound/vless.md`

**Changes**:
- Added `connection_pool` field to main structure
- Complete documentation for all 9 configuration options
- Added 4 usage examples:
  1. Low Latency (Gaming, Trading)
  2. High Availability (Production)
  3. Resource Efficient (Mobile, IoT)
  4. With Multiplex
- Compatibility warnings
- Default values for all fields
- Technical explanations

**Stats**: +156 lines

### 2. Chinese Documentation (中文)
**File**: `docs/configuration/outbound/vless.zh.md`

**Changes**:
- 添加 `connection_pool` 字段到主结构
- 9 个配置选项的完整文档
- 添加 4 个使用示例:
  1. 低延迟（游戏、交易）
  2. 高可用性（生产环境）
  3. 资源高效（移动设备、IoT）
  4. 与多路复用一起使用
- 兼容性警告
- 所有字段的默认值
- 技术说明

**Stats**: +156 lines

## Configuration Fields Documented

1. **ensure_idle_session** - Number of idle connections to maintain
2. **ensure_idle_session_create_rate** - Max connections per cycle
3. **min_idle_session** - Minimum idle connections to keep
4. **min_idle_session_for_age** - Minimum idle for age-based cleanup
5. **idle_session_check_interval** - Maintenance cycle frequency
6. **idle_session_timeout** - Idle timeout duration
7. **max_connection_lifetime** - Maximum connection age
8. **connection_lifetime_jitter** - Lifetime jitter
9. **heartbeat** - TCP-level keepalive interval

## Documentation Features

### Comprehensive Coverage
- ✅ All fields documented with descriptions
- ✅ Default values specified
- ✅ Type information included
- ✅ Compatibility notes (tcp_fast_open)

### Real-World Examples
- ✅ Low latency configuration (gaming, trading)
- ✅ High availability configuration (production)
- ✅ Resource efficient configuration (mobile, IoT)
- ✅ Multiplex compatibility example

### Technical Details
- ✅ TCP keepalive vs VLESS heartbeat explained
- ✅ Maintenance algorithm described
- ✅ Thundering herd prevention explained
- ✅ Pool and multiplex relationship clarified

## Commit Information

**Commit Hash**: 94b11311
**Branch**: master
**Status**: Pushed to origin

**Commit Message**:
```
docs: Add VLESS connection pool documentation

Add comprehensive documentation for the new VLESS connection pool
feature in both English and Chinese versions.

Documentation includes:
- Configuration structure and all fields
- Detailed field descriptions with defaults
- Compatibility warnings (tcp_fast_open)
- Usage examples for different scenarios:
  * Low latency (gaming, trading)
  * High availability (production)
  * Resource efficient (mobile, IoT)
  * With multiplex
- Technical explanations

Files updated:
- docs/configuration/outbound/vless.md
- docs/configuration/outbound/vless.zh.md

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>
```

## Release Notes Updated

**Release**: v1.12.14.9
**URL**: https://github.com/KexiChanProjectProxy/sing-box/releases/tag/v1.12.14.9

**Added Section**:
```markdown
## Documentation

Complete documentation has been added for the VLESS connection pool feature:

- **English**: [VLESS Outbound Configuration](https://sing-box.sagernet.org/configuration/outbound/vless/)
- **中文**: [VLESS 出站配置](https://sing-box.sagernet.org/zh/configuration/outbound/vless/)

Documentation includes:
- All configuration fields with detailed descriptions
- Default values and compatibility warnings
- Usage examples for different scenarios
- Technical explanations

Commit: 94b11311
```

## Statistics

- **Total Changes**: 312 insertions(+), 2 deletions(-)
- **Languages**: 2 (English, 中文)
- **Files Modified**: 2
- **Examples Added**: 8 (4 per language)
- **Fields Documented**: 9
- **Lines Added (English)**: 156
- **Lines Added (Chinese)**: 156

## Verification

### Documentation Links
- English: https://sing-box.sagernet.org/configuration/outbound/vless/
- Chinese: https://sing-box.sagernet.org/zh/configuration/outbound/vless/

### Repository Status
```
✅ Committed to master branch
✅ Pushed to GitHub origin
✅ Release notes updated
✅ Links verified
```

## Complete Release Timeline

1. **38b04fe0** - Feature implementation (Jan 24, 2026)
   - 1,855 lines of code
   - 18 comprehensive tests
   - Full implementation

2. **Release v1.12.14.9** - GitHub release published
   - 6 platform binaries
   - Comprehensive release notes
   - Download links

3. **94b11311** - Documentation update (Jan 26, 2026)
   - English documentation
   - Chinese documentation
   - Release notes updated

## Summary

The VLESS connection pool feature is now fully documented in both English and Chinese, with comprehensive coverage of all configuration options, real-world usage examples, and technical explanations. The documentation is committed to the repository and the release notes have been updated to include links to the new documentation.

**Status**: ✅ Complete
**Quality**: Production-ready
**Languages**: English + 中文
**Examples**: 8 real-world configurations
