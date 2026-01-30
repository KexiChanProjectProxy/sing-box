#!/bin/bash
set -e

VERSION="v1.12.14.9"
RELEASE_TITLE="sing-box v1.12.14.9 - VLESS Connection Pool"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║         sing-box v1.12.14.9 Release Process                 ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo

# Step 1: Check authentication
echo "Step 1/5: Checking GitHub authentication..."
if ! gh auth status >/dev/null 2>&1; then
    echo "❌ GitHub authentication required"
    echo "   Please run: gh auth login -h github.com"
    exit 1
fi
echo "✅ GitHub authenticated"
echo

# Step 2: Push code
echo "Step 2/5: Pushing code to master..."
if git push origin master 2>&1; then
    echo "✅ Code pushed to master"
else
    echo "⚠️  Push may have failed or already up to date"
fi
echo

# Step 3: Create and push tag
echo "Step 3/5: Creating and pushing git tag..."
if git tag "$VERSION" 2>/dev/null; then
    echo "✅ Tag $VERSION created"
else
    echo "⚠️  Tag already exists, continuing..."
fi

if git push origin "$VERSION" 2>&1; then
    echo "✅ Tag pushed to origin"
else
    echo "⚠️  Tag may already exist on remote"
fi
echo

# Step 4: Create GitHub release
echo "Step 4/5: Creating GitHub release..."
echo "   Title: $RELEASE_TITLE"
echo "   Tag: $VERSION"
echo "   Uploading 6 binaries..."
echo

gh release create "$VERSION" \
    --title "$RELEASE_TITLE" \
    --notes-file RELEASE_NOTES_v1.12.14.9.md \
    releases/sing-box-linux-amd64#"sing-box-linux-amd64" \
    releases/sing-box-linux-arm64#"sing-box-linux-arm64" \
    releases/sing-box-windows-amd64.exe#"sing-box-windows-amd64.exe" \
    releases/sing-box-windows-arm64.exe#"sing-box-windows-arm64.exe" \
    releases/sing-box-darwin-amd64#"sing-box-darwin-amd64" \
    releases/sing-box-darwin-arm64#"sing-box-darwin-arm64"

echo
echo "✅ Release created successfully!"
echo

# Step 5: Display release info
echo "Step 5/5: Release information..."
RELEASE_URL=$(gh release view "$VERSION" --json url --jq .url)
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🎉 Release $VERSION published successfully!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo
echo "📦 Release URL:"
echo "   $RELEASE_URL"
echo
echo "📥 Download binaries:"
echo "   Linux (amd64):   gh release download $VERSION -p 'sing-box-linux-amd64'"
echo "   Linux (arm64):   gh release download $VERSION -p 'sing-box-linux-arm64'"
echo "   Windows (amd64): gh release download $VERSION -p 'sing-box-windows-amd64.exe'"
echo "   Windows (arm64): gh release download $VERSION -p 'sing-box-windows-arm64.exe'"
echo "   macOS (amd64):   gh release download $VERSION -p 'sing-box-darwin-amd64'"
echo "   macOS (arm64):   gh release download $VERSION -p 'sing-box-darwin-arm64'"
echo
echo "✨ All done!"
