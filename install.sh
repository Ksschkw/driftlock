#!/bin/sh
set -e

REPO="Ksschkw/driftlock"
BIN_NAME="driftlock"
DEFAULT_PREFIX="/usr/local/bin"

# ---------- helpers ----------
info()  { printf '\033[1;32m[driftlock-install]\033[0m %s\n' "$1"; }
warn()  { printf '\033[1;33m[driftlock-install]\033[0m %s\n' "$1" >&2; }
die()   { warn "$1"; exit 1; }

# ---------- detect OS / arch ----------
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
  linux)  os="linux" ;;
  darwin) os="darwin" ;;
  *) die "Unsupported OS: $OS" ;;
esac

case "$ARCH" in
  x86_64|amd64)  arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) die "Unsupported architecture: $ARCH" ;;
esac

TARGET="${BIN_NAME}-${os}-${arch}"
if [ "$os" = "windows" ]; then
  TARGET="${TARGET}.exe"
fi

# ---------- fetch latest release tag ----------
info "Fetching latest release..."
TAG=$(curl -s https://api.github.com/repos/${REPO}/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
[ -z "$TAG" ] && die "Could not determine latest release tag."

DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${TARGET}"
CHECKSUM_URL="${DOWNLOAD_URL}.sha256"

# ---------- download ----------
TMPDIR=$(mktemp -d)
cd "$TMPDIR"

info "Downloading ${TARGET} from ${DOWNLOAD_URL} ..."
curl -sSL -o "${TARGET}" "$DOWNLOAD_URL" || die "Download failed."

# ---------- optional checksum verification ----------
VERIFIED=0
# First, try a per-asset .sha256 file
if curl -sSL -o "${TARGET}.sha256" "$CHECKSUM_URL" 2>/dev/null; then
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum -c "${TARGET}.sha256" --status 2>/dev/null && VERIFIED=1 || warn "Per-asset checksum mismatch."
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 -c "${TARGET}.sha256" --status 2>/dev/null && VERIFIED=1 || warn "Per-asset checksum mismatch."
  fi
fi

# Fallback: try checksums.txt containing multiple hashes
if [ "$VERIFIED" -eq 0 ]; then
  CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${TAG}/checksums.txt"
  if curl -sSL -o checksums.txt "$CHECKSUMS_URL" 2>/dev/null; then
    EXPECTED_HASH=$(grep " ${TARGET}$" checksums.txt | awk '{print $1}')
    if [ -n "$EXPECTED_HASH" ]; then
      ACTUAL_HASH=$(sha256sum "${TARGET}" 2>/dev/null | awk '{print $1}' || shasum -a 256 "${TARGET}" 2>/dev/null | awk '{print $1}')
      if [ "$EXPECTED_HASH" = "$ACTUAL_HASH" ]; then
        VERIFIED=1
        info "Checksum verified via checksums.txt."
      else
        warn "Checksum mismatch in checksums.txt."
      fi
    else
      warn "Asset not found in checksums.txt."
    fi
  fi
fi

if [ "$VERIFIED" -eq 0 ]; then
  warn "Checksum verification not available or failed. The binary may be corrupted."
  warn "To be safe, download it manually from https://github.com/${REPO}/releases/tag/${TAG}"
fi

# ---------- install ----------
PREFIX="${PREFIX:-$DEFAULT_PREFIX}"
if [ ! -d "$PREFIX" ]; then
  mkdir -p "$PREFIX" || die "Cannot create $PREFIX. Try setting PREFIX=\$HOME/.local/bin"
fi

INSTALL_PATH="${PREFIX}/${BIN_NAME}"
install -m 755 "${TARGET}" "$INSTALL_PATH" 2>/dev/null || {
  warn "install command not found, using cp..."
  cp -f "${TARGET}" "$INSTALL_PATH" && chmod +x "$INSTALL_PATH"
}

info "Driftlock installed to ${INSTALL_PATH}"

# ---------- PATH check ----------
case ":$PATH:" in
  *:"$PREFIX":*) ;;
  *)
    warn "Installation directory ${PREFIX} is not in your PATH."
    warn "Add 'export PATH=\$PATH:${PREFIX}' to your shell profile and restart your terminal."
    ;;
esac

cd /
rm -rf "$TMPDIR"
info "Done. Run 'driftlock init' inside a Git repository to get started."