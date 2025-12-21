#!/bin/bash

# This script compiles the Rust library as a Universal Binary for macOS (arm64 + x86_64)

set -e

# Get the directory of the script
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"

# Confirm OS
OS=$(uname -s)
if [ "$OS" != "Darwin" ]; then
    echo "❌ This script is intended for macOS only."
    exit 1
fi

TARGET_DIR="$PROJECT_ROOT/pkg/embedder/lib/darwin"
mkdir -p "$TARGET_DIR"

cd "$PROJECT_ROOT/embed_anything_binding"

# Check if rustup is available to add targets
if command -v rustup >/dev/null 2>&1; then
    echo "🦀 Adding Rust targets for macOS..."
    rustup target add aarch64-apple-darwin x86_64-apple-darwin
else
    echo "⚠️  rustup not found. Assuming targets are already installed."
fi

echo "🍎 Building for Apple Silicon (arm64)..."
cargo build --release --target aarch64-apple-darwin

echo "💻 Building for Intel (x86_64)..."
cargo build --release --target x86_64-apple-darwin

if command -v lipo >/dev/null 2>&1; then
    echo "🚀 Creating Universal Binary using lipo..."
    lipo -create \
        target/aarch64-apple-darwin/release/libembed_anything_binding.a \
        target/x86_64-apple-darwin/release/libembed_anything_binding.a \
        -output "$TARGET_DIR/libembed_anything_binding.a"
    
    # Copy ONNX Runtime shared library
    find target -name "libonnxruntime.dylib*" -exec cp {} "$TARGET_DIR/" \;
    echo "✅ Universal library created at $TARGET_DIR/libembed_anything_binding.a"
    lipo -info "$TARGET_DIR/libembed_anything_binding.a"
else
    echo "❌ lipo not found. Unable to create universal binary."
    echo "Falling back to host architecture only..."
    cp target/release/libembed_anything_binding.a "$TARGET_DIR/"
fi
