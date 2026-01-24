#!/bin/bash
set -e

VERSION="1.12.14.9"
RELEASE_DIR="releases"
# Full feature build tags for desktop platforms
TAGS="with_acme,with_clash_api,with_dhcp,with_embedded_tor,with_grpc,with_gvisor,with_low_memory,with_quic,with_shadowsocksr,with_utls,with_wireguard,with_tailscale"

# Build function
build_binary() {
    GOOS=$1
    GOARCH=$2
    OUTPUT_NAME="sing-box-${GOOS}-${GOARCH}"

    if [ "$GOOS" = "windows" ]; then
        OUTPUT_NAME="${OUTPUT_NAME}.exe"
    fi

    echo "Building $OUTPUT_NAME with full features..."
    CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build \
        -tags "$TAGS" \
        -trimpath \
        -ldflags "-s -w -buildid=" \
        -o "$RELEASE_DIR/$OUTPUT_NAME" \
        ./cmd/sing-box
    
    if [ $? -eq 0 ]; then
        echo "✓ Built $OUTPUT_NAME ($(du -h "$RELEASE_DIR/$OUTPUT_NAME" | cut -f1))"
    else
        echo "✗ Failed to build $OUTPUT_NAME"
        return 1
    fi
}

echo "Building release binaries for v${VERSION}..."
echo

build_binary linux amd64
build_binary linux arm64
build_binary windows amd64
build_binary windows arm64
build_binary darwin amd64
build_binary darwin arm64

echo
echo "Build complete! Binaries in $RELEASE_DIR/"
ls -lh "$RELEASE_DIR/"
