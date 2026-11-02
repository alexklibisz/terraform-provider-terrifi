#!/usr/bin/env bash
#
# Install a pre-release version of the terrifi provider from GitHub Releases.
#
# Usage:
#   ./install.sh              # installs the latest release
#   ./install.sh v0.11.0-RC0  # installs a specific version
#
# This downloads the provider binary and places it in a local filesystem mirror
# so OpenTofu can discover it without a registry. Run from the directory
# containing your configuration. For Terraform, set
# TERRIFI_REGISTRY_HOST=registry.terraform.io.
#
set -euo pipefail

REPO="alexklibisz/terraform-provider-terrifi"
VERSION="${1:-}"
REGISTRY_HOST="${TERRIFI_REGISTRY_HOST:-registry.opentofu.org}"
MIRROR_ROOT="${TERRIFI_MIRROR_DIR:-$PWD/terraform.d/plugins}"

# Detect OS and architecture.
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  darwin|linux) ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# If no version specified, fetch the latest release tag.
if [ -z "$VERSION" ]; then
  VERSION="$(gh release list --repo "$REPO" --limit 1 --json tagName --jq '.[0].tagName')"
  if [ -z "$VERSION" ]; then
    echo "No releases found for $REPO" >&2
    exit 1
  fi
  echo "Latest release: $VERSION"
fi

VERSION_NUMBER="${VERSION#v}"
ASSET="terraform-provider-terrifi_${VERSION_NUMBER}_${OS}_${ARCH}.zip"

# Download and extract.
DOWNLOAD_DIR="$(mktemp -d)"
trap 'rm -rf "$DOWNLOAD_DIR"' EXIT
echo "Downloading $ASSET..."
gh release download "$VERSION" --repo "$REPO" --pattern "$ASSET" --dir "$DOWNLOAD_DIR"
unzip -o "$DOWNLOAD_DIR/$ASSET" -d "$DOWNLOAD_DIR"

# Use the unpacked layout to preserve case-sensitive prerelease versions such
# as RC0: OpenTofu lowercases ZIP filenames when discovering packed mirrors.
MIRROR_DIR="$MIRROR_ROOT/$REGISTRY_HOST/alexklibisz/terrifi/${VERSION_NUMBER}/${OS}_${ARCH}"
mkdir -p "$MIRROR_DIR"
mv "$DOWNLOAD_DIR/terraform-provider-terrifi" "$MIRROR_DIR/"
chmod +x "$MIRROR_DIR/terraform-provider-terrifi"

echo "Installed terrifi $VERSION to $MIRROR_DIR"
echo ""
echo "Pin this version in your required_providers block:"
echo ""
echo '  terrifi = {'
echo '    source  = "alexklibisz/terrifi"'
echo "    version = \"= $VERSION_NUMBER\""
echo '  }'
echo ""
echo "With default CLI settings, terraform.d/plugins is discovered automatically."
echo "If you configure provider_installation explicitly, add a filesystem_mirror"
echo "for $MIRROR_ROOT and exclude $REGISTRY_HOST/alexklibisz/terrifi from direct."
