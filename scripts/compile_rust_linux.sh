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
# Download and copy ONNX Runtime shared library
ORT_VERSION="1.22.0"
ORT_ARCH="x64"
if [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
    ORT_ARCH="aarch64"
fi

echo "⬇️  Downloading ONNX Runtime v$ORT_VERSION..."
curl -L "https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/onnxruntime-linux-${ORT_ARCH}-${ORT_VERSION}.tgz" \
| tar xz

# Copy to target dir (including symlinks)
cp -P onnxruntime-linux-${ORT_ARCH}-${ORT_VERSION}/lib/libonnxruntime.so* "$TARGET_DIR/"
rm -rf onnxruntime-linux-${ORT_ARCH}-${ORT_VERSION}

echo "✅ Library compiled and copied to $TARGET_DIR"
