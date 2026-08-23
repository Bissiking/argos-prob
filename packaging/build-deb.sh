#!/usr/bin/env bash
set -euo pipefail

# build-deb.sh — Create a .deb package for argos-prob
# Usage: build-deb.sh <build_dir> <version> <arch>

BUILD_DIR="${1:?Usage: $0 <build_dir> <version> <arch>}"
VERSION="${2:?Usage: $0 <build_dir> <version> <arch>}"
ARCH="${3:?Usage: $0 <build_dir> <version> <arch>}"

APP_NAME="argos-prob"
DEB_NAME="${APP_NAME}_${VERSION}_${ARCH}"
DEB_DIR="${BUILD_DIR}/${DEB_NAME}"
PACKAGE_DIR="${BUILD_DIR}/packages"

mkdir -p "${PACKAGE_DIR}"

# ── Clean previous staging ──
rm -rf "${DEB_DIR}"

# ── Directory structure ──
mkdir -p "${DEB_DIR}/DEBIAN"
mkdir -p "${DEB_DIR}/usr/bin"
mkdir -p "${DEB_DIR}/etc/argos-prob"
mkdir -p "${DEB_DIR}/lib/systemd/system"

# ── Binary ──
cp "${BUILD_DIR}/${APP_NAME}-linux-${ARCH}" "${DEB_DIR}/usr/bin/${APP_NAME}"
chmod 755 "${DEB_DIR}/usr/bin/${APP_NAME}"

# ── Default config ──
cat > "${DEB_DIR}/etc/argos-prob/config.json" <<'CONF'
{
  "master_url": "https://argos-master.example.com",
  "listen_addr": ":9101",
  "mode": "passive",
  "interval": 60
}
CONF

# ── Systemd service ──
cat > "${DEB_DIR}/lib/systemd/system/${APP_NAME}.service" <<'UNIT'
[Unit]
Description=Argos Prob - Host monitoring agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/bin/argos-prob run
Restart=on-failure
RestartSec=10
ProtectSystem=strict
ProtectHome=true
NoNewPrivileges=true
ReadOnlyPaths=/etc/argos-prob
ReadWritePaths=/var/lib/argos-prob
StateDirectory=argos-prob

[Install]
WantedBy=multi-user.target
UNIT

# ── Postinst: enable + start service ──
cat > "${DEB_DIR}/DEBIAN/postinst" <<'POSTINST'
#!/bin/bash
set -e

if [ "$1" = "configure" ]; then
    systemctl daemon-reload
    systemctl enable argos-prob.service
    systemctl start argos-prob.service
fi
POSTINST
chmod 755 "${DEB_DIR}/DEBIAN/postinst"

# ── Prerm: stop service ──
cat > "${DEB_DIR}/DEBIAN/prerm" <<'PRERM'
#!/bin/bash
set -e

if [ "$1" = "remove" ] || [ "$1" = "deconfigure" ]; then
    systemctl stop argos-prob.service || true
    systemctl disable argos-prob.service || true
fi
PRERM
chmod 755 "${DEB_DIR}/DEBIAN/prerm"

# ── Postrm: cleanup after removal ──
cat > "${DEB_DIR}/DEBIAN/postrm" <<'POSTRM'
#!/bin/bash
set -e

if [ "$1" = "purge" ]; then
    systemctl daemon-reload
    rm -rf /etc/argos-prob
    rm -rf /var/lib/argos-prob
fi
POSTRM
chmod 755 "${DEB_DIR}/DEBIAN/postrm"

# ── control file ──
cat > "${DEB_DIR}/DEBIAN/control" <<CTRL
Package: ${APP_NAME}
Version: ${VERSION}
Section: admin
Priority: optional
Architecture: ${ARCH}
Depends: adduser
Maintainer: Argos Team <dev@argos.example.com>
Description: Argos Prob - Host monitoring agent
 Argos Prob collects host metrics (CPU, memory, disk, network, systemd
 services, Docker containers, Proxmox VMs) and reports them to an
 Argos master server.
CTRL

# ── Build ──
if command -v dpkg-deb &>/dev/null; then
    dpkg-deb --build --root-owner-group "${DEB_DIR}" "${PACKAGE_DIR}/${DEB_NAME}.deb"
    echo " -> ${PACKAGE_DIR}/${DEB_NAME}.deb"
elif command -v docker &>/dev/null; then
    echo "dpkg-deb not found locally, using Docker..."
    docker run --rm \
        -v "$(cd "${BUILD_DIR}" && pwd)":/build \
        debian:bookworm \
        dpkg-deb --build --root-owner-group \
        "/build/${DEB_NAME}" \
        "/build/packages/${DEB_NAME}.deb"
    echo " -> ${PACKAGE_DIR}/${DEB_NAME}.deb"
else
    echo "ERROR: Neither dpkg-deb nor Docker found." >&2
    echo "Install one of:" >&2
    echo "  brew install dpkg" >&2
    echo "  OR install Docker: https://docs.docker.com/get-docker/" >&2
    exit 1
fi
