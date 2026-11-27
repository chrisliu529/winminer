#!/bin/sh
set -eu

case "$PLATFORM_NAME" in
  iphoneos)
    sdk=iphoneos
    target="arm64-apple-ios${IPHONEOS_DEPLOYMENT_TARGET}"
    ;;
  iphonesimulator)
    sdk=iphonesimulator
    target="arm64-apple-ios${IPHONEOS_DEPLOYMENT_TARGET}-simulator"
    ;;
  *)
    echo "Unsupported iOS platform: $PLATFORM_NAME" >&2
    exit 1
    ;;
esac

sdk_path="$(xcrun --sdk "$sdk" --show-sdk-path)"
clang="$(xcrun --sdk "$sdk" --find clang)"
source_root="$(cd "$SRCROOT/../.." && pwd)"
output="$DERIVED_FILE_DIR/libwinminer.a"

mkdir -p "$DERIVED_FILE_DIR"
cd "$source_root"
GOOS=ios GOARCH=arm64 CGO_ENABLED=1 \
  CC="$clang -target $target -isysroot $sdk_path" \
  go build -buildmode=c-archive -o "$output" .
