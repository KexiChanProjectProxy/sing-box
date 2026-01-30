# Release Workflow

## The CORRECT Way to Release

**CRITICAL**: Always follow this exact order to ensure binaries match the committed code:

### Workflow Steps

1. **Write code and test it**
2. **Commit code changes** (WITHOUT version bump)
3. **Use the release script** to handle everything else automatically

### Using the Release Script

```bash
# Syntax:
./release.sh <version> "<commit message>"

# Example:
./release.sh 1.12.14.20 "fix: Hash ruleset regex matching works correctly"
```

The script will automatically:
- ✓ Check for uncommitted changes (fails if you have uncommitted code)
- ✓ Update version files (constant/version.go, build-releases.sh)
- ✓ Commit version bump with your message
- ✓ Build binaries FROM the committed code
- ✓ Verify binary revision matches commit
- ✓ Push to GitHub
- ✓ Create GitHub release (uses release-notes-vX.X.X.md if exists)
- ✓ Download and verify the released binary

### Manual Workflow (NOT RECOMMENDED)

If you must do it manually, follow this EXACT order:

1. **Commit your code changes first**:
   ```bash
   git add route/route.go route/router.go  # Your changed files
   git commit -m "fix: Your fix description"
   ```

2. **Bump version and commit**:
   ```bash
   # Edit constant/version.go
   # Edit build-releases.sh
   git add constant/version.go build-releases.sh
   git commit -m "chore: Bump version to 1.12.14.X"
   ```

3. **Build binaries** (AFTER commit):
   ```bash
   ./build-releases.sh
   ```

4. **Verify binary revision**:
   ```bash
   EXPECTED=$(git rev-parse HEAD)
   ACTUAL=$(./releases/sing-box-linux-amd64 version | grep Revision | awk '{print $2}')
   if [[ "$EXPECTED" != "$ACTUAL" ]]; then
       echo "ERROR: Revision mismatch!"
       exit 1
   fi
   ```

5. **Push to GitHub**:
   ```bash
   git push origin master
   ```

6. **Create release**:
   ```bash
   gh release create vX.X.X \
     --title "vX.X.X - Title" \
     --notes-file release-notes-vX.X.X.md \
     releases/*
   ```

7. **Verify downloaded binary**:
   ```bash
   wget https://github.com/.../sing-box-linux-amd64
   ./sing-box-linux-amd64 version | grep Revision
   # Should match git commit hash
   ```

## Common Mistakes to AVOID

### ❌ WRONG: Build before commit
```bash
./build-releases.sh           # ❌ Built from working tree
git commit -m "fix: something"  # ❌ Committed AFTER build
# Result: Binary has old commit hash!
```

### ✓ CORRECT: Commit before build
```bash
git commit -m "fix: something"  # ✓ Commit FIRST
./build-releases.sh            # ✓ Build AFTER commit
# Result: Binary has correct commit hash
```

### ❌ WRONG: Update version in code changes commit
```bash
# Edit code + version in same commit
git commit -m "fix: something"
# Result: Hard to track version bumps in git history
```

### ✓ CORRECT: Separate version bump commit
```bash
git commit -m "fix: something"        # Code changes
git commit -m "chore: Bump to X.X.X"  # Version bump
# Result: Clean git history
```

## Troubleshooting

### Binary has wrong revision hash

**Cause**: You built binaries before committing code.

**Fix**:
```bash
# Rebuild after commit
./build-releases.sh

# Verify
./releases/sing-box-linux-amd64 version | grep Revision
git rev-parse HEAD
# These should match!
```

### Release has wrong binaries

**Cause**: You uploaded binaries built from wrong commit.

**Fix**:
```bash
# Delete bad release
gh release delete vX.X.X --yes

# Rebuild and re-upload
./build-releases.sh
gh release create vX.X.X --notes-file release-notes.md releases/*
```

## Why This Matters

When you build before committing:
- Binary reports version X.X.X (from constant/version.go)
- But binary has revision hash from PREVIOUS commit
- User downloads "v1.12.14.19" but gets code from v1.12.14.17
- **Result**: Released binary doesn't contain the fix you just wrote!

## Verification Checklist

Before announcing a release, verify:

- [ ] `git log -1` shows the version bump commit
- [ ] `./releases/sing-box-linux-amd64 version` shows correct version
- [ ] Binary revision matches `git rev-parse HEAD`
- [ ] GitHub release exists
- [ ] Download from GitHub and verify revision again
- [ ] Test the downloaded binary (not your local build!)
