#!/bin/bash
set -e

VERSION=${WITNZ_VERSION:-latest}
INSTALL_DIR=${WITNZ_INSTALL_DIR:-/usr/local/bin}
GITHUB_REPO="witnz/witnz"

detect_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)

    case "$arch" in
        x86_64|amd64)
            arch="amd64"
            ;;
        aarch64|arm64)
            arch="arm64"
            ;;
        *)
            echo "Error: Unsupported architecture: $arch"
            exit 1
            ;;
    esac

    case "$os" in
        linux)
            os="linux"
            ;;
        darwin)
            os="darwin"
            ;;
        *)
            echo "Error: Unsupported OS: $os"
            exit 1
            ;;
    esac

    echo "${os}-${arch}"
}

main() {
    echo "Installing witnz..."
    echo ""

    PLATFORM=$(detect_platform)
    echo "Detected platform: $PLATFORM"

    if [ "$VERSION" = "latest" ]; then
        DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/latest/download/witnz-${PLATFORM}"
    else
        DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}/witnz-${PLATFORM}"
    fi

    echo "Downloading from: $DOWNLOAD_URL"

    TMP_FILE=$(mktemp)
    if command -v curl > /dev/null 2>&1; then
        curl -sSL "$DOWNLOAD_URL" -o "$TMP_FILE"
    elif command -v wget > /dev/null 2>&1; then
        wget -q "$DOWNLOAD_URL" -O "$TMP_FILE"
    else
        echo "Error: curl or wget is required"
        exit 1
    fi

    chmod +x "$TMP_FILE"

    echo ""
    echo "Installing to ${INSTALL_DIR}/witnz..."

    if [ -w "$INSTALL_DIR" ]; then
        mv "$TMP_FILE" "${INSTALL_DIR}/witnz"
    else
        echo "Requesting sudo permissions to install to ${INSTALL_DIR}..."
        sudo mv "$TMP_FILE" "${INSTALL_DIR}/witnz"
    fi

    echo ""
    echo "✓ witnz installed successfully!"
    echo ""
    echo "Run 'witnz version' to verify installation"
    echo ""
    echo "Get started:"
    echo "  witnz init --config witnz.yaml"
    echo "  witnz start --config witnz.yaml"
    echo ""
    echo "Documentation: https://github.com/${GITHUB_REPO}"
}

main
