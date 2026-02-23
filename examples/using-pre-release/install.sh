#!/usr/bin/env bash
#
# Install a pre-release version of the terrifi provider from GitHub Releases.
#
# Usage:
#   ./install.sh              # installs the latest release
#   ./install.sh v0.1.0-RC1   # installs a specific version
#
# This downloads the provider binary and places it in a local filesystem mirror
# so Terraform can discover it without a registry.
#
set -euo pipefail

REPO="alexklibisz/terrifi"
VERSION="${1:-}"

# Detect OS and architecture.
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# If no version specified, fetch the latest release tag.
if [ -z "$VERSION" ]; then
  VERSION="$(gh release list --repo "$REPO" --limit 1 --json tagName --jq '.[0].tagName')"
  echo "Latest release: $VERSION"
fi

VERSION_NUMBER="${VERSION#v}"
ASSET="terrifi_${VERSION_NUMBER}_${OS}_${ARCH}.zip"

# Download and extract.
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT
echo "Downloading $ASSET..."
gh release download "$VERSION" --repo "$REPO" --pattern "$ASSET" --dir "$TMPDIR"
unzip -o "$TMPDIR/$ASSET" -d "$TMPDIR"

# Install into a local filesystem mirror that Terraform can discover.
MIRROR_DIR="$HOME/.terraform.d/plugins/github.com/alexklibisz/terrifi/${VERSION_NUMBER}/${OS}_${ARCH}"
mkdir -p "$MIRROR_DIR"
mv "$TMPDIR/terraform-provider-terrifi" "$MIRROR_DIR/"
chmod +x "$MIRROR_DIR/terraform-provider-terrifi"

echo "Installed terrifi $VERSION to $MIRROR_DIR"
echo ""
echo "Add this to your ~/.terraformrc:"
echo ""
echo '  provider_installation {'
echo '    filesystem_mirror {'
echo '      path = "'$HOME'/.terraform.d/plugins"'
echo '    }'
echo '    direct {}'
echo '  }'
