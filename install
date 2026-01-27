#!/bin/bash
set -e

# zen-lock installation script
# This script installs the zen-lock CLI binary

VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="zen-lock"

# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

# Map architecture
case "$ARCH" in
  x86_64)
    ARCH="amd64"
    ;;
  arm64|aarch64)
    ARCH="arm64"
    ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

# Map OS
case "$OS" in
  linux)
    OS="linux"
    ;;
  darwin)
    OS="darwin"
    ;;
  *)
    echo "Unsupported OS: $OS"
    exit 1
    ;;
esac

echo "Installing zen-lock CLI..."
echo "  OS: $OS"
echo "  Architecture: $ARCH"
echo "  Version: $VERSION"
echo "  Install directory: $INSTALL_DIR"

# Determine download URL
if [ "$VERSION" = "latest" ]; then
  DOWNLOAD_URL="https://github.com/kube-zen/zen-lock/releases/latest/download/zen-lock-${OS}-${ARCH}"
else
  DOWNLOAD_URL="https://github.com/kube-zen/zen-lock/releases/download/${VERSION}/zen-lock-${OS}-${ARCH}"
fi

# Create temp directory
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

# Download binary
echo "Downloading zen-lock from $DOWNLOAD_URL..."
if command -v curl >/dev/null 2>&1; then
  curl -fsSL -o "$TMP_DIR/$BINARY_NAME" "$DOWNLOAD_URL"
elif command -v wget >/dev/null 2>&1; then
  wget -q -O "$TMP_DIR/$BINARY_NAME" "$DOWNLOAD_URL"
else
  echo "Error: curl or wget is required"
  exit 1
fi

# Make binary executable
chmod +x "$TMP_DIR/$BINARY_NAME"

# Install binary
if [ ! -w "$INSTALL_DIR" ]; then
  echo "Requires sudo to install to $INSTALL_DIR"
  sudo mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
else
  mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
fi

# Verify installation
if command -v "$BINARY_NAME" >/dev/null 2>&1; then
  INSTALLED_VERSION=$($BINARY_NAME version 2>/dev/null || echo "unknown")
  echo "✅ zen-lock installed successfully!"
  echo "   Version: $INSTALLED_VERSION"
  echo "   Location: $(which $BINARY_NAME)"
else
  echo "❌ Installation failed: binary not found in PATH"
  exit 1
fi

