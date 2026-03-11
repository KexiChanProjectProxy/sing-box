#!/usr/bin/env bash
set -e

VERSION=$(grep -oP 'var Version = "\K[^"]+' constant/version.go)
TAGS_OTHERS="$(cat release/DEFAULT_BUILD_TAGS_OTHERS)"
TAGS_WINDOWS="$(cat release/DEFAULT_BUILD_TAGS_WINDOWS)"
LDFLAGS_SHARED="$(cat release/LDFLAGS)"
LDFLAGS="-X 'github.com/sagernet/sing-box/constant.Version=${VERSION}' ${LDFLAGS_SHARED} -s -w -buildid="
OUT_DIR="releases_new"
mkdir -p "$OUT_DIR"

build() {
    local GOOS=$1 GOARCH=$2 EXT=$3 TAGS=$4
    local OUT="${OUT_DIR}/sing-box-${VERSION}-${GOOS}-${GOARCH}${EXT}"
    echo "Building ${GOOS}/${GOARCH}..."
    CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH \
        go build -v -trimpath \
        -ldflags "$LDFLAGS" \
        -tags "$TAGS" \
        -o "$OUT" \
        ./cmd/sing-box
    echo "  => $OUT ($(du -h "$OUT" | cut -f1))"
}

# Desktop platforms
build linux   amd64 ""     "$TAGS_OTHERS"  &
build linux   arm64 ""     "$TAGS_OTHERS"  &
build darwin  amd64 ""     "$TAGS_OTHERS"  &
build darwin  arm64 ""     "$TAGS_OTHERS"  &
build windows amd64 ".exe" "$TAGS_WINDOWS" &
build windows arm64 ".exe" "$TAGS_WINDOWS" &

wait
echo "All desktop builds complete."

# Android APK (modern, arm64-v8a only)
bash "$(dirname "$0")/build_android.sh"

cd "$OUT_DIR" && sha256sum sing-box-${VERSION}-* SFA-${VERSION}-* > checksums.txt
echo "Checksums written to ${OUT_DIR}/checksums.txt"
ls -lh
