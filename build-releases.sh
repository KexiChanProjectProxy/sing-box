#!/bin/bash
set -e

# Read version from constant/version.go
VERSION=$(grep -oP 'var Version = "\K[^"]+' constant/version.go)
RELEASE_DIR="releases"
# Full feature build tags for desktop platforms
TAGS="with_gvisor,with_quic,with_dhcp,with_wireguard,with_utls,with_acme,with_clash_api,with_tailscale,with_embedded_tor,with_grpc,with_shadowsocksr"

# Clean and create release directory
rm -rf "$RELEASE_DIR"
mkdir -p "$RELEASE_DIR"

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
        -v \
        -tags "$TAGS" \
        -trimpath \
        -ldflags "-s -w -buildid= -X github.com/sagernet/sing-box/constant.Version=${VERSION}" \
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
echo "Generating checksums..."
cd "$RELEASE_DIR"
sha256sum * | tee checksums.txt
cd ..

echo
echo "Build complete! Binaries in $RELEASE_DIR/"
ls -lh "$RELEASE_DIR/"
