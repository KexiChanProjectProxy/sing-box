#!/bin/bash
set -e

# Proper release workflow script
# Usage: ./release.sh <version> "<commit message>"
# Example: ./release.sh 1.12.14.20 "fix: Some bug fix"

if [ $# -lt 2 ]; then
    echo "Usage: ./release.sh <version> \"<commit message>\""
    echo "Example: ./release.sh 1.12.14.20 \"fix: Some bug fix\""
    exit 1
fi

VERSION=$1
COMMIT_MSG=$2

echo "=========================================="
echo "Release Workflow for v${VERSION}"
echo "=========================================="

# Step 1: Check for uncommitted changes
echo ""
echo "Step 1: Checking for uncommitted changes..."
if ! git diff-index --quiet HEAD --; then
    echo "ERROR: You have uncommitted changes!"
    echo "Please commit your code changes FIRST before releasing."
    git status --short
    exit 1
fi
echo "✓ No uncommitted changes"

# Step 2: Update version in files
echo ""
echo "Step 2: Updating version to ${VERSION}..."
sed -i "s/var Version = .*/var Version = \"${VERSION}\"/" constant/version.go
sed -i "s/VERSION=.*/VERSION=\"${VERSION}\"/" build-releases.sh
echo "✓ Updated constant/version.go and build-releases.sh"

# Step 3: Commit version bump
echo ""
echo "Step 3: Committing version bump..."
git add constant/version.go build-releases.sh
git commit -m "chore: Bump version to ${VERSION}" -m "${COMMIT_MSG}" -m "Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>"
echo "✓ Version bump committed"

# Step 4: Get commit hash
COMMIT_HASH=$(git rev-parse HEAD)
SHORT_HASH=$(git rev-parse --short HEAD)
echo ""
echo "Step 4: Current commit: ${COMMIT_HASH}"

# Step 5: Build binaries FROM the committed code
echo ""
echo "Step 5: Building binaries from commit ${SHORT_HASH}..."
./build-releases.sh
echo "✓ Binaries built"

# Step 6: Verify binary has correct revision
echo ""
echo "Step 6: Verifying binary revision..."
BINARY_REVISION=$(./releases/sing-box-linux-amd64 version | grep Revision | awk '{print $2}')
if [[ ! "$BINARY_REVISION" == "$COMMIT_HASH" ]]; then
    echo "ERROR: Binary revision mismatch!"
    echo "  Expected: ${COMMIT_HASH}"
    echo "  Got:      ${BINARY_REVISION}"
    exit 1
fi
echo "✓ Binary revision matches commit: ${SHORT_HASH}"

# Step 7: Push to GitHub
echo ""
echo "Step 7: Pushing to GitHub..."
git push origin master
echo "✓ Pushed to GitHub"

# Step 8: Create GitHub release
echo ""
echo "Step 8: Creating GitHub release..."

# Check if release notes file exists
RELEASE_NOTES="release-notes-v${VERSION}.md"
if [ ! -f "$RELEASE_NOTES" ]; then
    echo "WARNING: ${RELEASE_NOTES} not found!"
    echo "Creating basic release notes..."
    cat > "$RELEASE_NOTES" << EOF
# sing-box v${VERSION}

${COMMIT_MSG}

## Build Information

- **Version**: ${VERSION}
- **Build Date**: $(date +%Y-%m-%d)
- **Commit**: ${SHORT_HASH}
- **Build Tags**: \`with_acme,with_clash_api,with_dhcp,with_embedded_tor,with_grpc,with_gvisor,with_low_memory,with_quic,with_shadowsocksr,with_utls,with_wireguard,with_tailscale\`

## Download

All binaries built with full feature support.

### Platforms

- Linux: amd64, arm64
- Windows: amd64, arm64
- macOS: amd64 (Intel), arm64 (Apple Silicon)
EOF
fi

# Delete existing release if it exists
if gh release view "v${VERSION}" &>/dev/null; then
    echo "Deleting existing release v${VERSION}..."
    gh release delete "v${VERSION}" --yes
fi

# Create release
gh release create "v${VERSION}" \
  --title "v${VERSION}" \
  --notes-file "${RELEASE_NOTES}" \
  releases/sing-box-linux-amd64 \
  releases/sing-box-linux-arm64 \
  releases/sing-box-windows-amd64.exe \
  releases/sing-box-windows-arm64.exe \
  releases/sing-box-darwin-amd64 \
  releases/sing-box-darwin-arm64

echo "✓ GitHub release created"

# Step 9: Verify release
echo ""
echo "Step 9: Verifying release..."
RELEASE_URL=$(gh release view "v${VERSION}" --json url --jq '.url')
echo "✓ Release URL: ${RELEASE_URL}"

# Step 10: Final verification
echo ""
echo "Step 10: Final verification..."
echo "Downloading and checking binary from release..."
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"
wget -q "https://github.com/KexiChanProjectProxy/sing-box/releases/download/v${VERSION}/sing-box-linux-amd64"
chmod +x sing-box-linux-amd64
DOWNLOAD_REVISION=$(./sing-box-linux-amd64 version | grep Revision | awk '{print $2}')
cd - > /dev/null
rm -rf "$TEMP_DIR"

if [[ ! "$DOWNLOAD_REVISION" == "$COMMIT_HASH" ]]; then
    echo "ERROR: Downloaded binary revision mismatch!"
    echo "  Expected: ${COMMIT_HASH}"
    echo "  Got:      ${DOWNLOAD_REVISION}"
    exit 1
fi
echo "✓ Downloaded binary verified: ${SHORT_HASH}"

echo ""
echo "=========================================="
echo "✓ Release v${VERSION} complete!"
echo "=========================================="
echo ""
echo "Summary:"
echo "  Version: ${VERSION}"
echo "  Commit:  ${SHORT_HASH}"
echo "  Release: ${RELEASE_URL}"
echo ""
echo "The release has been published and verified."
