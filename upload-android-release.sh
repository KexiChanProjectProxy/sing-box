#!/bin/bash
set -e

# Get version from git tag
VERSION=$(go run github.com/sagernet/sing-box/cmd/internal/read_tag@latest 2>&1 | grep -v "^go:")

RELEASE_DIR="releases/android"
RELEASE_TAG="v${VERSION}"

echo "Uploading Android binaries for ${RELEASE_TAG}..."
echo

# Check if release directory exists and has files
if [ ! -d "$RELEASE_DIR" ] || [ -z "$(ls -A $RELEASE_DIR 2>/dev/null)" ]; then
    echo "Error: No binaries found in $RELEASE_DIR"
    echo "Please run ./build-android.sh first"
    exit 1
fi

# List files to be uploaded
echo "Files to upload:"
ls -lh "$RELEASE_DIR"
echo

# Upload to GitHub releases
# Using --replace to replace existing files if the release already exists
# Using --draft to create/update as draft release
# Using --prerelease to mark as pre-release
echo "Uploading to GitHub release ${RELEASE_TAG}..."
GOPATH=$(go env GOPATH)
"${GOPATH}/bin/ghr" --replace --draft --prerelease -p 3 "${RELEASE_TAG}" "${RELEASE_DIR}"

echo
echo "✓ Upload complete!"
echo "Visit: https://github.com/$(git config --get remote.origin.url | sed 's/.*github.com[:/]\(.*\)\.git/\1/')/releases"
