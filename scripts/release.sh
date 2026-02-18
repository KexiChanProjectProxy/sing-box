#!/bin/bash
# Release preparation script
# This script helps prepare a new release by:
# 1. Updating version in constant/version.go
# 2. Moving unreleased changelog entries to the new version
# 3. Creating and pushing a git tag to trigger the release workflow

set -e

# Check if version is provided
if [ -z "$1" ]; then
    echo "Usage: $0 <version> [date]"
    echo "Example: $0 1.12.14.30"
    echo "Example: $0 1.12.14.30 2026-02-19"
    exit 1
fi

VERSION="$1"
DATE="${2:-$(date +%Y-%m-%d)}"

echo "Preparing release $VERSION for $DATE..."

# Check if CHANGELOG.md exists
if [ ! -f "CHANGELOG.md" ]; then
    echo "Error: CHANGELOG.md not found"
    exit 1
fi

# Step 1: Update version in constant/version.go
echo "Updating version in constant/version.go..."
sed -i "s/var Version = \".*\"/var Version = \"$VERSION\"/" constant/version.go

# Step 2: Update CHANGELOG.md - add new version section
echo "Updating CHANGELOG.md..."

# Create a temporary file with the new version section
awk -v version="$VERSION" -v date="$DATE" '
/^## \[Unreleased\]/ {
    print
    print ""
    print "---"
    print ""
    print "## [" version "] - " date
    print ""
    next
}
{print}
' CHANGELOG.md > CHANGELOG.md.tmp && mv CHANGELOG.md.tmp CHANGELOG.md

# Step 3: Show git diff
echo ""
echo "Changes to be committed:"
echo "======================="
git diff --no-color constant/version.go | head -20
echo ""
git diff --no-color CHANGELOG.md | head -40

# Step 4: Ask for confirmation
echo ""
echo "Ready to commit and tag version $VERSION"
read -p "Continue? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted"
    git restore constant/version.go CHANGELOG.md
    exit 1
fi

# Step 5: Commit changes
echo ""
echo "Committing changes..."
git add constant/version.go CHANGELOG.md
git commit -m "chore: Release $VERSION"

# Step 6: Create and push tag
echo ""
echo "Creating and pushing tag v$VERSION..."
git tag -a "v$VERSION" -m "Release $VERSION"
git push origin master
git push origin "v$VERSION"

echo ""
echo "Release $VERSION prepared and tagged!"
echo "GitHub Actions will now build and publish the release."
echo "Release will be available at: https://github.com/KexiChanProjectProxy/sing-box/releases/tag/v$VERSION"
