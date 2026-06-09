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
DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"

echo "Detected: ${OS}/${ARCH}"
echo "Downloading ${ASSET}..."

mkdir -p "$INSTALL_DIR"

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$DOWNLOAD_URL" -o "$INSTALL_DIR/$BIN_NAME"
elif command -v wget >/dev/null 2>&1; then
  wget -qO "$INSTALL_DIR/$BIN_NAME" "$DOWNLOAD_URL"
else
  echo "Error: neither curl nor wget found. Please install one and retry." >&2
  exit 1
fi

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
