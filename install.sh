#!/bin/sh
# Installs the latest hexplore release binary for your OS/arch.
#
#   curl -fsSL https://raw.githubusercontent.com/juniorsaldanha/hexplore/main/install.sh | sh
#
# Set INSTALL_DIR to change the install location (default: ~/.local/bin).
# Set VERSION to install a specific tag instead of the latest (e.g. v0.3.0).
set -eu

REPO="juniorsaldanha/hexplore"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

os() {
	case "$(uname -s)" in
	Linux) echo linux ;;
	Darwin) echo darwin ;;
	*)
		echo "hexplore: unsupported OS: $(uname -s) — see https://github.com/$REPO/releases for a Windows build" >&2
		exit 1
		;;
	esac
}

arch() {
	case "$(uname -m)" in
	x86_64 | amd64) echo amd64 ;;
	arm64 | aarch64) echo arm64 ;;
	*)
		echo "hexplore: unsupported architecture: $(uname -m)" >&2
		exit 1
		;;
	esac
}

OS="$(os)"
ARCH="$(arch)"

if [ -z "${VERSION:-}" ]; then
	VERSION="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
fi
if [ -z "$VERSION" ]; then
	echo "hexplore: couldn't determine the latest version — set VERSION=vX.Y.Z and retry" >&2
	exit 1
fi

ARCHIVE="hexplore_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/$REPO/releases/download/$VERSION"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "hexplore: downloading $ARCHIVE ($VERSION)..."
curl -fsSL -o "$TMP/$ARCHIVE" "$BASE_URL/$ARCHIVE"
curl -fsSL -o "$TMP/checksums.txt" "$BASE_URL/checksums.txt"

echo "hexplore: verifying checksum..."
(
	cd "$TMP"
	if command -v sha256sum >/dev/null 2>&1; then
		grep " $ARCHIVE\$" checksums.txt | sha256sum -c -
	else
		grep " $ARCHIVE\$" checksums.txt | shasum -a 256 -c -
	fi
)

tar -xzf "$TMP/$ARCHIVE" -C "$TMP" hexplore
mkdir -p "$INSTALL_DIR"
install -m 755 "$TMP/hexplore" "$INSTALL_DIR/hexplore"

echo "hexplore: installed to $INSTALL_DIR/hexplore"
case ":$PATH:" in
*":$INSTALL_DIR:"*) ;;
*) echo "hexplore: $INSTALL_DIR is not on your PATH — add it, e.g.: export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
esac
