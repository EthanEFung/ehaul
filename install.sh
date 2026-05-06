#!/bin/sh
set -e

REPO="EthanEFung/ehaul"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
    darwin|linux) ;;
    *) echo "Error: unsupported OS: $OS" >&2; exit 1 ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64)          ARCH="amd64" ;;
    aarch64|arm64)   ARCH="arm64" ;;
    *) echo "Error: unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Get latest version
TAG="$(curl -fsSL https://api.github.com/repos/$REPO/releases/latest | grep '"tag_name"' | cut -d'"' -f4)"
VERSION="${TAG#v}"

# Download and install
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

ARCHIVE="ehaul_${VERSION}_${OS}_${ARCH}.tar.gz"
curl -fsSL "https://github.com/$REPO/releases/download/$TAG/$ARCHIVE" -o "$WORK/$ARCHIVE"
tar xzf "$WORK/$ARCHIVE" -C "$WORK"
install -m 755 "$WORK/ehaul" "$INSTALL_DIR/ehaul"

echo "ehaul $TAG installed to $INSTALL_DIR/ehaul"
