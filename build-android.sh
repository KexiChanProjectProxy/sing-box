#!/bin/bash
set -e

VERSION="1.12.14.6"
RELEASE_DIR="releases/android"
# Android-compatible build tags
TAGS="with_gvisor,with_quic,with_dhcp,with_wireguard,with_utls,with_acme,with_clash_api,with_tailscale"

# Create output directory
mkdir -p "$RELEASE_DIR"

# Build function for Android
build_android_binary() {
    GOARCH=$1
    OUTPUT_NAME="sing-box-android-${GOARCH}"

    echo "Building $OUTPUT_NAME..."

    # Some architectures require CGO, try without CGO first, then with if needed
    if CGO_ENABLED=0 GOOS=android GOARCH=$GOARCH go build \
        -tags "$TAGS" \
        -trimpath \
        -ldflags "-s -w -buildid=" \
        -o "$RELEASE_DIR/$OUTPUT_NAME" \
        ./cmd/sing-box 2>/dev/null; then
        echo "✓ Built $OUTPUT_NAME ($(du -h "$RELEASE_DIR/$OUTPUT_NAME" | cut -f1))"
    else
        echo "⚠ $GOARCH requires CGO, skipping (or set up Android NDK for CGO builds)"
        return 0
    fi
}

echo "Building Android binaries for v${VERSION}..."
echo

# Build for common Android architectures
build_android_binary arm64   # 64-bit ARM (most modern devices)
build_android_binary arm     # 32-bit ARM (older devices) - may require CGO
build_android_binary amd64   # x86_64 (emulators, some devices)
build_android_binary 386     # 32-bit x86 (older emulators) - may require CGO

echo
echo "Build complete! Binaries in $RELEASE_DIR/"
ls -lh "$RELEASE_DIR/"
