#!/bin/bash
set -e

# GitHub repository
REPO="KexiChanProjectProxy/sing-box"

# Read version from constant/version.go
VERSION=$(grep -oP 'var Version = "\K[^"]+' constant/version.go)
RELEASE_DIR="releases"

echo "Publishing release v${VERSION} to GitHub..."

# Check if release directory exists
if [ ! -d "$RELEASE_DIR" ]; then
    echo "Error: Release directory not found. Run ./build-releases.sh first."
    exit 1
fi

# Check if checksums.txt exists
if [ ! -f "$RELEASE_DIR/checksums.txt" ]; then
    echo "Error: checksums.txt not found in $RELEASE_DIR"
    exit 1
fi

# Extract changelog for this version
echo "Extracting changelog..."
if [ -f "CHANGELOG.md" ]; then
    # Extract content between ## [version] and the next ## or end of file
    CHANGELOG=$(awk "/## \[$VERSION\]/,/^## / {print} /^## / && !first {first=1; next}" CHANGELOG.md | sed '1d;$d')

    if [ -z "$CHANGELOG" ]; then
        CHANGELOG="Release $VERSION"
    fi

    # Get title from the first line
    TITLE=$(grep "^## \[$VERSION\]" CHANGELOG.md | sed "s/## \[$VERSION\] //")

    if [ -z "$TITLE" ]; then
        TITLE="Release $VERSION"
    fi
else
    CHANGELOG="Release $VERSION"
    TITLE="Release $VERSION"
fi

echo "Release title: $TITLE"
echo "Changelog:"
echo "$CHANGELOG"
echo
echo "Files to upload:"
ls -lh "$RELEASE_DIR"
echo

# Ask for confirmation
read -p "Create and publish release v$VERSION? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted"
    exit 1
fi

# Check if release already exists
if gh release view "v$VERSION" --repo "$REPO" >/dev/null 2>&1; then
    echo "Release v$VERSION already exists. Deleting old release..."
    gh release delete "v$VERSION" --repo "$REPO" --yes
    # Also delete the tag if it exists
    git push origin ":refs/tags/v$VERSION" || true
fi

# Create the release
echo "Creating GitHub release..."
gh release create "v$VERSION" \
    --repo "$REPO" \
    --title "$TITLE" \
    --notes "$CHANGELOG" \
    "$RELEASE_DIR"/*

echo
echo "Release v$VERSION published successfully!"
echo "View at: https://github.com/$REPO/releases/tag/v$VERSION"
