#!/bin/bash

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}Installing Kimchi...${NC}"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
darwin*) OS="darwin" ;;
linux*) OS="linux" ;;
*)
	echo -e "${RED}Unsupported OS: $OS${NC}" >&2
	exit 1
	;;
esac

ARCH=$(uname -m)
case "$ARCH" in
x86_64) ARCH="amd64" ;;
aarch64 | arm64) ARCH="arm64" ;;
*)
	echo -e "${RED}Unsupported architecture: $ARCH${NC}" >&2
	exit 1
	;;
esac

BINARY_URL="https://github.com/castai/kimchi/releases/latest/download/kimchi_${OS}_${ARCH}.tar.gz"

TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

echo -e "${BLUE}Downloading kimchi for ${OS}/${ARCH}...${NC}"
if ! curl -fsSL "$BINARY_URL" | tar -xzf - -C "$TEMP_DIR"; then
	echo -e "${RED}Failed to download kimchi${NC}" >&2
	echo "Please check that the release exists at:"
	echo "  https://github.com/castai/kimchi/releases"
	exit 1
fi

chmod +x "$TEMP_DIR/kimchi"

if [ -w /usr/local/bin ]; then
	mv "$TEMP_DIR/kimchi" /usr/local/bin/kimchi
	INSTALL_PATH="/usr/local/bin/kimchi"
else
	mkdir -p ~/.local/bin
	mv "$TEMP_DIR/kimchi" ~/.local/bin/kimchi
	INSTALL_PATH="$HOME/.local/bin/kimchi"
	echo ""
	echo -e "${BLUE}Note: Add ~/.local/bin to your PATH:${NC}"
	echo "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.bashrc"
	echo "  source ~/.bashrc"
fi

echo ""
echo -e "${GREEN}✓ Installed kimchi to ${INSTALL_PATH}${NC}"
echo ""
echo -e "${BLUE}Launching Kimchi...${NC}"
echo ""
exec "$INSTALL_PATH"
