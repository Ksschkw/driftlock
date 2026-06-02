#!/usr/bin/env bash
# install.sh – install driftlock binary and set up PATH
set -e

BIN_DIR="/usr/local/bin"
BINARY_NAME="driftlock"

# Determine OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    armv7l)  ARCH="armv7" ;;
esac

VERSION="0.1.0"  # update on each release
REPO="Ksschkw/driftlock"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${BINARY_NAME}-${OS}-${ARCH}"

echo "Downloading driftlock v${VERSION} for ${OS}/${ARCH}..."
curl -fsSL "$URL" -o "/tmp/${BINARY_NAME}"
sha256=$(curl -fsSL "${URL}.sha256" | cut -d ' ' -f 1)
echo "${sha256}  /tmp/${BINARY_NAME}" | sha256sum --check
chmod +x "/tmp/${BINARY_NAME}"

echo "Installing to ${BIN_DIR}/${BINARY_NAME}..."
sudo mv "/tmp/${BINARY_NAME}" "${BIN_DIR}/${BINARY_NAME}"

echo "Driftlock installed successfully."
echo "Test it with: driftlock --help"