# Release Workflow

This directory contains scripts and documentation for the automated release workflow.

## Automatic Release Process

The project uses GitHub Actions to automatically build and publish releases when a new version tag is pushed.

### How it Works

1. **Trigger**: Push a tag matching `v*.*.*.*` (e.g., `v1.12.14.30`)
2. **Extract Changelog**: The workflow extracts the changelog for this version from `CHANGELOG.md`
3. **Build**: Binaries are built for:
   - linux (amd64, arm64)
   - windows (amd64, arm64)
   - darwin (amd64, arm64)
4. **Release**: GitHub release is created with:
   - Changelog as release notes
   - All build binaries
   - SHA256 checksums

### Creating a Release

Use the `release.sh` script to prepare a new release:

```bash
./scripts/release.sh 1.12.14.30
```

The script will:
1. Update version in `constant/version.go`
2. Move unreleased changes to the new version in `CHANGELOG.md`
3. Commit the changes
4. Create and push the git tag

### Manual Release Process

If you prefer to do it manually:

1. **Update Version** in `constant/version.go`:
   ```go
   var Version = "1.12.14.30"
   ```

2. **Update CHANGELOG.md**:
   - Move entries from `[Unreleased]` to the new version section
   - Add the release date

3. **Commit**:
   ```bash
   git add constant/version.go CHANGELOG.md
   git commit -m "chore: Release 1.12.14.30"
   ```

4. **Tag and Push**:
   ```bash
   git tag -a v1.12.14.30 -m "Release 1.12.14.30"
   git push origin master
   git push origin v1.12.14.30
   ```

### CHANGELOG Format

The CHANGELOG.md follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) format:

```markdown
## [Unreleased]

### Added
<!-- New features go here -->

### Fixed
<!-- Bug fixes go here -->

---

## [1.12.14.30] - 2026-02-19

### Added
- **Feature**: Description

### Fixed
- **Bug**: Description
```

### Claude AI Integration

When working with Claude AI on this project:

1. **Add Changes to Unreleased Section**: Before committing, document your changes in the `[Unreleased]` section of `CHANGELOG.md` using the appropriate categories (Added, Changed, Fixed, etc.)

2. **Example Entry**:
   ```markdown
   ## [Unreleased]

   ### Added
   - **LoadBalance**: Added configurable `tolerance` field for Top-N selection
     - Previous candidates within tolerance ms remain eligible
     - Reduces hash ring rebuilds for consistent_hash

   ### Changed
   - **DNS**: Improved resolver performance

   ### Fixed
   - **Bug**: Fixed memory leak in connection pool
   ```

3. **When Creating Release**: Use `./scripts/release.sh` or manually move the `[Unreleased]` entries to a new version section.

### Workflow Files

- **`.github/workflows/release.yml`**: Main release workflow
  - Triggers on tag push
  - Extracts changelog
  - Builds binaries
  - Creates GitHub release

- **`scripts/release.sh`**: Helper script for release preparation
  - Updates version
  - Updates CHANGELOG
  - Creates and pushes tag

### Build Tags

Release binaries are built with the following tags:
- `with_gvisor`
- `with_quic`
- `with_dhcp`
- `with_wireguard`
- `with_utls`
- `with_acme`
- `with_clash_api`
- `with_tailscale`
- `with_embedded_tor`
- `with_grpc`
- `with_shadowsocksr`
