# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

## Template for Future Releases

## [X.Y.Z] - YYYY-MM-DD

### Added
- **Feature Area**: Brief description of new feature

### Changed
- **Component**: Brief description of what changed

### Deprecated
- **Feature**: Brief description of deprecation (if applicable)

### Removed
- **Feature**: Brief description of what was removed

### Fixed
- **Bug**: Brief description of fix

### Security
- **Issue**: Brief description of security fix
