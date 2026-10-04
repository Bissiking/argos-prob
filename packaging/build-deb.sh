#!/usr/bin/env bash
set -euo pipefail

# Linux/macOS builder. Windows can use packaging/build-linux.py instead.
BUILD_DIR="${1:?Usage: $0 <build_dir> <version> <arch>}"
VERSION="${2:?Usage: $0 <build_dir> <version> <arch>}"
ARCH="${3:?Usage: $0 <build_dir> <version> <arch>}"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Invalid version" >&2; exit 1; }
[[ "$ARCH" == amd64 || "$ARCH" == arm64 ]] || { echo "Invalid architecture" >&2; exit 1; }
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
APP_NAME=argos-prob
DEB_NAME="${APP_NAME}_${VERSION}_${ARCH}"
mkdir -p "$BUILD_DIR/packages"
DEB_DIR="$(mktemp -d "$BUILD_DIR/${DEB_NAME}.XXXXXX")"
trap 'rm -rf "$DEB_DIR"' EXIT
install -d "$DEB_DIR/DEBIAN" "$DEB_DIR/usr/bin" "$DEB_DIR/lib/systemd/system"
install -d -m 700 "$DEB_DIR/etc/argos-prob"
install -m 755 "$BUILD_DIR/${APP_NAME}-linux-${ARCH}" "$DEB_DIR/usr/bin/argos-prob"
install -m 644 "$SCRIPT_DIR/argos-prob.service" "$DEB_DIR/lib/systemd/system/argos-prob.service"
for script in preinst postinst prerm postrm; do
    install -m 755 "$SCRIPT_DIR/debian/$script" "$DEB_DIR/DEBIAN/$script"
done
# Configuration belongs to the operator: never package a replacement config.json.
cat > "$DEB_DIR/DEBIAN/control" <<CTRL
Package: argos-prob
Version: ${VERSION}
Section: admin
Priority: optional
Architecture: ${ARCH}
Maintainer: Argos Team <dev@argos.example.com>
Description: Argos Prob - Host monitoring agent
 Collects host metrics and controlled resource inventories for Argos.
CTRL
if command -v dpkg-deb >/dev/null; then
    dpkg-deb --build --root-owner-group "$DEB_DIR" "$BUILD_DIR/packages/${DEB_NAME}.deb"
elif command -v docker >/dev/null; then
    docker run --rm -v "$(cd "$BUILD_DIR" && pwd)":/build debian:bookworm \
        dpkg-deb --build --root-owner-group "/build/$(basename "$DEB_DIR")" "/build/packages/${DEB_NAME}.deb"
else
    echo "dpkg-deb or Docker required; alternatively use: python packaging/build-linux.py" >&2
    exit 1
fi
echo " -> $BUILD_DIR/packages/${DEB_NAME}.deb"
