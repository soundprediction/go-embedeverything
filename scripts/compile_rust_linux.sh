#!/bin/bash

# This script compiles the Rust library for Linux (native build).
# It produces .so files and compresses them with gzip.

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
        ORT_ARCH="x64"
        ;;
    aarch64|arm64)
        PLATFORM="linux-arm64"
        ORT_ARCH="aarch64"
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

# Release build
cargo build --release

# Copy the shared library
# Note: CDYLIB produces .so on Linux
if [ -f "target/release/libembed_anything_binding.so" ]; then
    cp target/release/libembed_anything_binding.so "$TARGET_DIR/"
    echo "   Compressing binary..."
    gzip -9 -f "$TARGET_DIR/libembed_anything_binding.so"
else
    echo "❌ Build failed? Could not find target/release/libembed_anything_binding.so"
    exit 1
fi

# Download and copy ONNX Runtime shared library
ORT_VERSION="1.22.0"
ORT_TGZ="onnxruntime-linux-${ORT_ARCH}-${ORT_VERSION}.tgz"
ORT_URL="https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/${ORT_TGZ}"

echo "⬇️  Downloading ONNX Runtime v$ORT_VERSION..."
# Check if we already have the compressed lib
if [ ! -f "$TARGET_DIR/libonnxruntime.so.gz" ]; then
    curl -L -o "$ORT_TGZ" "$ORT_URL"
    tar xz -f "$ORT_TGZ"
    
    # Copy to target dir (resolve symlinks to real file)
    # We copy to current dir first to manipulate
    cp -P onnxruntime-linux-${ORT_ARCH}-${ORT_VERSION}/lib/libonnxruntime.so* "$TARGET_DIR/"
    
    # Cleanup download
    rm -rf onnxruntime-linux-${ORT_ARCH}-${ORT_VERSION}
    rm "$ORT_TGZ"

    # Go to target dir to compress
    pushd "$TARGET_DIR" > /dev/null
    
    # Rename versioned file to generic name if needed, or just keep generic
    # For simplicity in embedder.go, we look for libonnxruntime.so.gz
    # The tarball contains libonnxruntime.so -> libonnxruntime.so.1.22.0
    # We want the real file named libonnxruntime.so
    
    if [ -f "libonnxruntime.so.${ORT_VERSION}" ]; then
        mv "libonnxruntime.so.${ORT_VERSION}" "libonnxruntime.so"
    fi
    # Remove symlinks
    find . -name "libonnxruntime.so.*" -delete
    
    echo "   Compressing ONNX Runtime..."
    gzip -9 -f "libonnxruntime.so"
    popd > /dev/null
else
    echo "   ONNX Runtime seems present."
fi

echo "✅ Library compiled and copied to $TARGET_DIR"
