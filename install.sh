#!/bin/sh
# skill-inspector installer
# Usage: curl -fsSL https://raw.githubusercontent.com/parksjr/skill-inspector/main/install.sh | sh
set -e

REPO="parksjr/skill-inspector"
BIN_NAME="skill-inspector"
INSTALL_DIR="${SKILL_INSPECTOR_INSTALL_DIR:-$HOME/.local/bin}"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  darwin|linux) ;;
  *) echo "Error: unsupported OS: $OS" >&2; exit 1 ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)        ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "Error: unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

ASSET="${BIN_NAME}-${OS}-${ARCH}"
CHECKSUMS_FILE="sha256sums.txt"
DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/latest/download/${CHECKSUMS_FILE}"

echo "Detected: ${OS}/${ARCH}"
echo "Downloading ${ASSET}..."

mkdir -p "$INSTALL_DIR"
tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT

# Download binary
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$DOWNLOAD_URL" -o "$tmp_dir/$BIN_NAME" || {
    echo "Error: failed to download ${ASSET}" >&2
    exit 1
  }
  curl -fsSL "$CHECKSUMS_URL" -o "$tmp_dir/$CHECKSUMS_FILE" || {
    echo "Error: failed to download checksums" >&2
    exit 1
  }
elif command -v wget >/dev/null 2>&1; then
  wget -qO "$tmp_dir/$BIN_NAME" "$DOWNLOAD_URL" || {
    echo "Error: failed to download ${ASSET}" >&2
    exit 1
  }
  wget -qO "$tmp_dir/$CHECKSUMS_FILE" "$CHECKSUMS_URL" || {
    echo "Error: failed to download checksums" >&2
    exit 1
  }
else
  echo "Error: neither curl nor wget found. Please install one and retry." >&2
  exit 1
fi

# Verify SHA256 checksum
echo "Verifying checksum..."
expected=$(grep "${ASSET}" "$tmp_dir/$CHECKSUMS_FILE" | awk '{print $1}')
if [ -z "$expected" ]; then
  echo "Error: checksum entry not found for ${ASSET}" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp_dir/$BIN_NAME" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  actual=$(shasum -a 256 "$tmp_dir/$BIN_NAME" | awk '{print $1}')
else
  echo "Error: no SHA256 tool found (sha256sum or shasum required)" >&2
  exit 1
fi

if [ "$expected" != "$actual" ]; then
  echo "Error: checksum mismatch for ${ASSET}" >&2
  echo "  expected: $expected" >&2
  echo "  got:      $actual" >&2
  exit 1
fi
echo "✓ Checksum verified"

# Install binary
mv -f "$tmp_dir/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"

chmod +x "$INSTALL_DIR/$BIN_NAME"

echo ""
echo "✓ skill-inspector installed to $INSTALL_DIR/$BIN_NAME"
echo ""

# Warn if install dir is not in PATH
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    echo "Note: $INSTALL_DIR is not in your PATH."
    echo "Add the following to your shell profile (~/.zshrc, ~/.bashrc, etc.):"
    echo ""
    echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
    echo ""
    ;;
esac
