#!/usr/bin/env bash
# install.sh — Install mm-repro binary from GitHub Releases (or build from source).
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/rohith0456/mattermost-support-package-repro/main/scripts/install.sh | bash
#   VERSION=v0.2.0 bash install.sh
#   INSTALL_DIR=/usr/local/bin bash install.sh
set -euo pipefail

BINARY="mm-repro"
REPO="rohith0456/mattermost-support-package-repro"
VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# ── Detect platform ────────────────────────────────────────────────────────────

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)          ARCH="amd64" ;;
    arm64|aarch64)   ARCH="arm64" ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

EXT="tar.gz"
if [ "$OS" = "windows" ]; then
    EXT="zip"
fi

echo "mm-repro installer"
echo "=================="
echo "Platform: $OS/$ARCH"
echo "Install:  $INSTALL_DIR"
echo ""

# ── Resolve version ────────────────────────────────────────────────────────────

if [ "$VERSION" = "latest" ]; then
    echo "Fetching latest release version..."
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
    if [ -z "$VERSION" ]; then
        echo "Could not determine latest version. Set VERSION=v0.x.x explicitly."
        echo "  Releases: https://github.com/${REPO}/releases"
        exit 1
    fi
fi

echo "Version:  $VERSION"
echo ""

# ── Try to download pre-built binary ──────────────────────────────────────────

ARCHIVE="${BINARY}_${VERSION#v}_${OS}_${ARCH}.${EXT}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Downloading $ARCHIVE..."
if curl -fsSL --retry 3 -o "$TMP_DIR/$ARCHIVE" "$DOWNLOAD_URL" 2>/dev/null; then
    echo "Extracting..."
    mkdir -p "$INSTALL_DIR"
    if [ "$EXT" = "zip" ]; then
        unzip -q "$TMP_DIR/$ARCHIVE" "$BINARY" -d "$INSTALL_DIR" 2>/dev/null || \
            { unzip -q "$TMP_DIR/$ARCHIVE" -d "$TMP_DIR" && mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/"; }
    else
        tar -xzf "$TMP_DIR/$ARCHIVE" -C "$INSTALL_DIR" "$BINARY" 2>/dev/null || \
            { tar -xzf "$TMP_DIR/$ARCHIVE" -C "$TMP_DIR" && mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/"; }
    fi
    chmod +x "$INSTALL_DIR/$BINARY"
    echo ""
    echo "Installed: $INSTALL_DIR/$BINARY"
else
    echo "Pre-built binary not found. Falling back to 'go install'..."
    echo ""
    if ! command -v go &>/dev/null; then
        echo "ERROR: Go is not installed and no pre-built binary was found."
        echo ""
        echo "Options:"
        echo "  1. Install Go 1.22+: https://go.dev/dl/"
        echo "  2. Download a binary manually: https://github.com/${REPO}/releases"
        exit 1
    fi
    GO_VERSION=$(go version | grep -oE '[0-9]+\.[0-9]+' | head -1)
    echo "Building from source with Go $GO_VERSION..."
    PKG="github.com/${REPO}/cmd/mm-repro"
    if [ "$VERSION" = "latest" ]; then
        go install "${PKG}@latest"
    else
        go install "${PKG}@${VERSION}"
    fi
    echo ""
    echo "Installed to: $(go env GOPATH)/bin/$BINARY"
    INSTALL_DIR="$(go env GOPATH)/bin"
fi

# ── PATH hint ─────────────────────────────────────────────────────────────────

if ! echo ":$PATH:" | grep -q ":$INSTALL_DIR:"; then
    echo ""
    echo "NOTE: $INSTALL_DIR is not in your PATH. Add it with:"
    echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
    echo ""
fi

echo "Run: $BINARY doctor"
echo ""
echo "Quick start:"
echo "  $BINARY init --support-package ./customer.zip"
echo "  cd generated-repro/<name>/"
echo "  make run"
