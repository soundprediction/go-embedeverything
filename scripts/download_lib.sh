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
        ASSET_NAME="libembed_anything_binding-darwin-universal.a" # Example
        ;;
    Linux)
        GOOS="linux"
        case $ARCH in
            x86_64)
                PLATFORM="linux-amd64"
                ASSET_NAME="libembed_anything_binding-linux-x86_64.a"
                ;;
            aarch64|arm64)
                PLATFORM="linux-arm64"
                ASSET_NAME="libembed_anything_binding-linux-aarch64.a"
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

# Note: This is a template. The user should update the download logic
# once they have a release pipeline.

# Example download logic:
# DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/$ASSET_NAME"
# curl -L -o "$TARGET_DIR/libembed_anything_binding.a" "$DOWNLOAD_URL"

# For now, we will just check if the library exists locally, 
# and if not, suggest running the compile script.

if [ ! -f "$TARGET_DIR/libembed_anything_binding.a" ]; then
    echo "⚠️  Library not found in $TARGET_DIR"
    if [ "$OS" = "Darwin" ]; then
        echo "💡 You can compile it locally by running: ./scripts/compile_rust_mac.sh"
    else
        echo "💡 You can compile it locally by running: ./scripts/compile_rust_linux.sh"
    fi
    # exit 1 # Uncomment to fail if download is mandatory
fi
