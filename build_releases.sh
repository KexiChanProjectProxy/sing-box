#!/usr/bin/env bash
set -e

VERSION="1.13.0.6"
TAGS="with_gvisor,with_quic,with_dhcp,with_wireguard,with_utls,with_acme,with_clash_api,with_tailscale,with_ccm,with_ocm,badlinkname,tfogo_checklinkname0"
LDFLAGS="-X 'github.com/sagernet/sing-box/constant.Version=${VERSION}' -X 'internal/godebug.defaultGODEBUG=multipathtcp=0' -s -w -buildid= -checklinkname=0"
OUT="release"

build() {
    local GOOS=$1 GOARCH=$2 EXT=$3
    local OUT_FILE="${OUT}/sing-box-${VERSION}-${GOOS}-${GOARCH}${EXT}"
    echo "Building ${GOOS}/${GOARCH}..."
    CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build -trimpath \
        -ldflags "$LDFLAGS" -tags "$TAGS" \
        -o "$OUT_FILE" ./cmd/sing-box
    echo "  => $OUT_FILE ($(du -h "$OUT_FILE" | cut -f1))"
}

build linux   amd64 ""     &
build linux   arm64 ""     &
build darwin  amd64 ""     &
build darwin  arm64 ""     &
build windows amd64 ".exe" &
build windows arm64 ".exe" &

wait
echo "All builds complete."
ls -lh "${OUT}/sing-box-${VERSION}-"*
