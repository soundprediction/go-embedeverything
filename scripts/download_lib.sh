#!/bin/bash

# This script downloads pre-compiled Rust libraries from GitHub releases.
# It is intended to be called by 'go generate'.

set -e

# Configuration
REPO="soundprediction/go-embedeverything"
VERSION="latest" # Or a specific tag

# Get the directory of the script
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"

# Detect OS and Arch
OS=$(uname -s)
ARCH=$(uname -m)

case $OS in
    Darwin)
        GOOS="darwin"
        PLATFORM="darwin"
        ASSET_NAME="libembed_anything_binding-darwin-universal.tar.gz"
        ;;
    Linux)
        GOOS="linux"
        case $ARCH in
            x86_64)
                PLATFORM="linux-amd64"
                ASSET_NAME="libembed_anything_binding-linux-x86_64.tar.gz"
                ;;
            aarch64|arm64)
                PLATFORM="linux-arm64"
                ASSET_NAME="libembed_anything_binding-linux-aarch64.tar.gz"
                ;;
            *)
                echo "Unsupported architecture: $ARCH"
                exit 1
                ;;
        esac
        ;;
    *)
        echo "Unsupported OS: $OS"
        exit 1
        ;;
esac

TARGET_DIR="$PROJECT_ROOT/pkg/embedder/lib/$PLATFORM"
mkdir -p "$TARGET_DIR"

echo "🔍 Detected platform: $PLATFORM"
echo "📦 This would download $ASSET_NAME from $REPO"

# Check if library exists. If not, try to download or suggest compilation.
if [ ! -f "$TARGET_DIR/libembed_anything_binding.a" ]; then
    echo "⚠️  Library not found in $TARGET_DIR"
    
    # Try to download if version is set (not just template)
    if [ "$VERSION" != "template" ]; then
        echo "🌐 Attempting to download pre-compiled binary ($ASSET_NAME)..."
        DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/$ASSET_NAME"
        
        # Temporary file for the tarball
        TEMP_TAR=$(mktemp)
        
        if command -v curl >/dev/null 2>&1; then
            if curl -L -f -o "$TEMP_TAR" "$DOWNLOAD_URL"; then
                tar -xzf "$TEMP_TAR" -C "$TARGET_DIR" --strip-components=0
                echo "✅ Downloaded and extracted library."
                rm "$TEMP_TAR"
                exit 0
            fi
        fi
        rm "$TEMP_TAR"
    fi

    echo "⚠️  Download failed or not attempted."
    
    # Check for cargo
    if command -v cargo >/dev/null 2>&1; then
        echo "🦀 Cargo found. Attempting to compile locally..."
        if [ "$OS" = "Darwin" ]; then
            sh "$SCRIPT_DIR/compile_rust_mac.sh"
        elif [ "$OS" = "Linux" ]; then
            sh "$SCRIPT_DIR/compile_rust_linux.sh"
        else
             echo "❌ Automatic compilation not supported for $OS"
             exit 1
        fi
        exit 0
    else
        echo "❌ Library not found and 'cargo' is not installed."
        echo "   Please install Rust/Cargo to compile locally, or ensure the binary release is available."
        exit 1
    fi
fi
