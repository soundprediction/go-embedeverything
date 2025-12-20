#!/bin/bash

# This script compiles the Rust library for Linux.

set -e

# Get the directory of the script
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"

# Confirm OS
OS=$(uname -s)
if [ "$OS" != "Linux" ]; then
    echo "❌ This script is intended for Linux only."
    exit 1
fi

ARCH=$(uname -m)
case $ARCH in
    x86_64)
        PLATFORM="linux-amd64"
        ;;
    aarch64|arm64)
        PLATFORM="linux-arm64"
        ;;
    *)
        echo "❌ Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

TARGET_DIR="$PROJECT_ROOT/pkg/embedder/lib/$PLATFORM"
mkdir -p "$TARGET_DIR"

echo "🦀 Building Rust library for $PLATFORM ($ARCH native)..."
cd "$PROJECT_ROOT/embed_anything_binding"

# Basic release build for the host
cargo build --release

# Copy the static library
cp target/release/libembed_anything_binding.a "$TARGET_DIR/"
# Copy ONNX Runtime shared library
find target -name "libonnxruntime.so*" -type f -exec cp {} "$TARGET_DIR/" \;

echo "✅ Library compiled and copied to $TARGET_DIR"
