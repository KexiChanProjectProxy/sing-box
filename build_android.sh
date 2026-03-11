#!/usr/bin/env bash
set -e

export ANDROID_HOME="$HOME/Android/Sdk"
export ANDROID_NDK_HOME="$ANDROID_HOME/ndk/28.0.13004108"
export JAVA_HOME="/usr/lib/jvm/java-21-openjdk-amd64"
export PATH="$PATH:$ANDROID_HOME/cmdline-tools/latest/bin:$ANDROID_HOME/platform-tools:$HOME/go/bin"

VERSION=$(grep -oP 'var Version = "\K[^"]+' constant/version.go)
SFA_DIR="../sing-box-for-android"
# SFA commit that matches libbox v1.13.x API
SFA_COMMIT="1ce499f"
OUT_DIR="releases_new"
mkdir -p "$OUT_DIR"

echo "Building Android APK for arm64-v8a (modern, API 23+), version ${VERSION}..."

# Ensure SFA is on the correct commit
git -C "$SFA_DIR" checkout "$SFA_COMMIT" 2>/dev/null || true

# Build libbox.aar for arm64 only, modern API 23
mkdir -p "${SFA_DIR}/app/libs"
go run ./cmd/internal/build_libbox -target android -platform android/arm64

# Update version.properties in the Android project
go run ./cmd/internal/update_android_version

# Build modern arm64-v8a APK only
"${SFA_DIR}/gradlew" -p "$SFA_DIR" :app:assembleOtherRelease
"${SFA_DIR}/gradlew" -p "$SFA_DIR" --stop

# Copy arm64-v8a APK
DEST="${OUT_DIR}/SFA-${VERSION}-arm64-v8a.apk"
cp "${SFA_DIR}/app/build/outputs/apk/other/release/SFA-${VERSION}-arm64-v8a.apk" "$DEST"
echo "  => $DEST ($(du -h "$DEST" | cut -f1))"
