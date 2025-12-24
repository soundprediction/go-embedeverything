#!/bin/bash
set -e

# This script attempts to cross-compile the Rust library for:
# - Linux x86_64 (via cargo-zigbuild)
# - Linux aarch64 (via cargo-zigbuild)
# - macOS Universal (x86_64 + arm64) (uses local tools)

# Ensure cargo is in PATH (standard location)
export PATH="$HOME/.cargo/bin:$PATH"

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"
RUST_DIR="$PROJECT_ROOT/embed_anything_binding"

# Function to check for required tools
check_tool() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "❌ Tool '$1' is required but not installed."
        if [ "$1" == "zig" ]; then
            echo "   Please install it with: brew install zig"
        elif [ "$1" == "cargo-zigbuild" ]; then
             echo "   Please install it with: cargo install cargo-zigbuild"
        fi
        exit 1
    fi
}

echo "🔍 Checking requirements..."
check_tool zig
check_tool cargo-zigbuild

# --- macOS Compilation ---
echo ""
echo "🍏 === Compiling for macOS (Universal) ==="
if [ "$(uname -s)" == "Darwin" ]; then
    MAC_SCRIPT="$SCRIPT_DIR/compile_rust_mac.sh"
    if [ -f "$MAC_SCRIPT" ]; then
        echo "   Calling $MAC_SCRIPT..."
        set +e
        bash "$MAC_SCRIPT"
        EXIT_CODE=$?
        set -e
        if [ $EXIT_CODE -ne 0 ]; then
            echo "⚠️  macOS build script failed. Continuing to Linux builds..."
        fi

        # Download and copy ONNX Runtime for macOS (Universal)
        MAC_DEST="$PROJECT_ROOT/pkg/embedder/lib/darwin"
        mkdir -p "$MAC_DEST"
        ORT_VERSION="1.22.0"
        ORT_TGZ_MAC="onnxruntime-osx-universal2-${ORT_VERSION}.tgz"
        ORT_URL_MAC="https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/${ORT_TGZ_MAC}"
        
        echo "   Downloading ONNX Runtime v${ORT_VERSION} for macOS (Universal)..."
        cd "$MAC_DEST"
        if [ ! -f "libonnxruntime.dylib.gz" ] && [ ! -f "libonnxruntime.dylib" ]; then
             curl -L -o "$ORT_TGZ_MAC" "$ORT_URL_MAC"
             echo "   Extracting..."
             tar xz -f "$ORT_TGZ_MAC"
             echo "   Copying libraries..."
             # Copy real file to avoid symlink issues with go:embed
             cp onnxruntime-osx-universal2-${ORT_VERSION}/lib/libonnxruntime.1.22.0.dylib libonnxruntime.dylib
             rm -rf "onnxruntime-osx-universal2-${ORT_VERSION}"
             rm "$ORT_TGZ_MAC"
             
             echo "   Compressing..."
             gzip -9 -f "libonnxruntime.dylib"
        else
            echo "   ONNX Runtime seems present, skipping download."
        fi
    else
        echo "❌ $MAC_SCRIPT not found! Skipping."
    fi
else
    echo "⚠️  Skipping macOS build (must be run on macOS)."
fi

# --- Linux Compilation ---
ORT_VERSION="1.22.0"

compile_linux() {
    local TARGET_ARCH="$1"      # e.g., x86_64 or aarch64
    local RUST_TARGET="$2"      # e.g., x86_64-unknown-linux-gnu
    local ORT_ARCH_NAME="$3"    # e.g., x64 or aarch64
    local PLATFORM_DIR="$4"     # e.g., linux-amd64 or linux-arm64

    echo ""
    echo "🐧 === Compiling for Linux ($TARGET_ARCH) ==="
    echo "   Target: $RUST_TARGET"
    echo "   Output: $PLATFORM_DIR"

    cd "$RUST_DIR"
    
    echo "   Running cargo zigbuild..."
    # Ensure release mode
    cargo zigbuild --release --target "$RUST_TARGET"

    # Destination directory
    local DEST="$PROJECT_ROOT/pkg/embedder/lib/$PLATFORM_DIR"
    mkdir -p "$DEST"

    # Copy shared lib
    # Note: cargo zigbuild/cross produces .so for cdylib
    local LIB_SRC="target/$RUST_TARGET/release/libembed_anything_binding.so"
    if [ -f "$LIB_SRC" ]; then
        echo "   Copying library to $DEST/"
        cp "$LIB_SRC" "$DEST/"
        echo "   Compressing..."
        gzip -9 -f "$DEST/libembed_anything_binding.so"
    else
        echo "❌ Build failed? Could not find $LIB_SRC"
        exit 1
    fi

    # Download and copy ONNX Runtime
    echo "   Downloading ONNX Runtime v${ORT_VERSION} for ${ORT_ARCH_NAME}..."
    cd "$DEST"
    local ORT_TGZ="onnxruntime-linux-${ORT_ARCH_NAME}-${ORT_VERSION}.tgz"
    local ORT_URL="https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/${ORT_TGZ}"
    
    # Check if we have the generic .so or compressed version
    if [ ! -f "libonnxruntime.so.gz" ] && [ ! -f "libonnxruntime.so" ]; then
        if [ ! -f "$ORT_TGZ" ]; then
             curl -L -o "$ORT_TGZ" "$ORT_URL"
        fi
        
        echo "   Extracting..."
        tar xz -f "$ORT_TGZ"
        
        echo "   Copying libraries..."
        # We need the real file to compress
        cp -P onnxruntime-linux-${ORT_ARCH_NAME}-${ORT_VERSION}/lib/libonnxruntime.so* .
        
        # Cleanup
        rm -rf "onnxruntime-linux-${ORT_ARCH_NAME}-${ORT_VERSION}"
        rm "$ORT_TGZ"

        # Compress libonnxruntime.so
        # Typically there are symlinks. We want the main file.
        # Let's resolve symlinks or just compress the main one.
        # Actually for simplicity, let's keep the .so.1.22.0 and name it libonnxruntime.so for our collection
        if [ -f "libonnxruntime.so.${ORT_VERSION}" ]; then
            mv "libonnxruntime.so.${ORT_VERSION}" "libonnxruntime.so"
        fi
        # Remove symlinks or other versions if any
        find . -name "libonnxruntime.so.*" -delete

        echo "   Compressing ORT..."
        gzip -9 -f "libonnxruntime.so"
    else
         echo "   ONNX Runtime seems present, skipping download."
    fi
    
    echo "✅ Linux ($TARGET_ARCH) done."
}

# Compile Linux AMD64
compile_linux "x86_64" "x86_64-unknown-linux-gnu" "x64" "linux-amd64"

# Compile Linux ARM64
compile_linux "aarch64" "aarch64-unknown-linux-gnu" "aarch64" "linux-arm64"

echo ""
echo "🎉 All cross-compilation tasks completed!"
