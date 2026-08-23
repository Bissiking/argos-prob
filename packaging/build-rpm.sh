#!/usr/bin/env bash
set -euo pipefail

# build-rpm.sh — Create an .rpm package for argos-prob
# Usage: build-rpm.sh <build_dir> <version> <arch>
#
# Requires: rpmbuild (install via: brew install rpm | apt install rpm)

BUILD_DIR="${1:?Usage: $0 <build_dir> <version> <arch>}"
VERSION="${2:?Usage: $0 <build_dir> <version> <arch>}"
ARCH="${3:?Usage: $0 <build_dir> <version> <arch>}"

APP_NAME="argos-prob"
PACKAGE_DIR="${BUILD_DIR}/packages"
RPM_TOPDIR="${BUILD_DIR}/rpm-build"
PACKAGE_DIR_RPM="${BUILD_DIR}/packages"

mkdir -p "${PACKAGE_DIR_RPM}"
rm -rf "${RPM_TOPDIR}"
mkdir -p "${RPM_TOPDIR}"/{BUILD,RPMS,SOURCES,SPECS,SRPMS}

# ── Create tarball as source ──
TARBALL="${APP_NAME}-${VERSION}.tar.gz"

# First, stage files for the tarball
STAGING="${RPM_TOPDIR}/staging"
rm -rf "${STAGING}"
mkdir -p "${STAGING}/${APP_NAME}-${VERSION}"
cp "${BUILD_DIR}/${APP_NAME}-linux-${ARCH}" "${STAGING}/${APP_NAME}-${VERSION}/${APP_NAME}"

# ── Write spec file ──
cat > "${RPM_TOPDIR}/SPECS/${APP_NAME}.spec" <<SPEC
%define debug_package %{nil}

Name:           ${APP_NAME}
Version:        ${VERSION}
Release:        1%{?dist}
Summary:        Argos Prob - Host monitoring agent

License:        Proprietary
URL:            https://github.com/Bissiking/argos-prob
Source0:        ${TARBALL}

%description
Argos Prob collects host metrics (CPU, memory, disk, network, systemd
services, Docker containers, Proxmox VMs) and reports them to an
Argos master server.

%prep
%setup -q -n ${APP_NAME}-${VERSION}

%install
mkdir -p %{buildroot}/usr/bin
mkdir -p %{buildroot}/etc/argos-prob
mkdir -p %{buildroot}/lib/systemd/system
install -m 755 ${APP_NAME} %{buildroot}/usr/bin/${APP_NAME}
install -m 644 config.json %{buildroot}/etc/argos-prob/config.json
install -m 644 ${APP_NAME}.service %{buildroot}/lib/systemd/system/${APP_NAME}.service

%post
systemctl daemon-reload
systemctl enable ${APP_NAME}.service
systemctl start ${APP_NAME}.service

%preun
if [ \$1 -eq 0 ]; then
    systemctl stop ${APP_NAME}.service || true
    systemctl disable ${APP_NAME}.service || true
fi

%postun
if [ \$1 -eq 0 ]; then
    systemctl daemon-reload
    rm -rf /etc/argos-prob
    rm -rf /var/lib/argos-prob
fi

%files
%defattr(-,root,root,-)
/usr/bin/${APP_NAME}
%config(noreplace) /etc/argos-prob/config.json
/lib/systemd/system/${APP_NAME}.service

%changelog
* $(date +"%a %b %d %Y") Argos Team <dev@argos.example.com> - ${VERSION}-1
- Release ${VERSION}
SPEC

# ── Prepare extra files for the tarball ──
# config.json
cat > "${STAGING}/${APP_NAME}-${VERSION}/config.json" <<'CONF'
{
  "master_url": "https://argos-master.example.com",
  "listen_addr": ":9101",
  "mode": "passive",
  "interval": 60
}
CONF

# systemd service
cat > "${STAGING}/${APP_NAME}-${VERSION}/${APP_NAME}.service" <<'UNIT'
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

# Create tarball
tar czf "${RPM_TOPDIR}/SOURCES/${TARBALL}" \
    -C "${STAGING}" \
    "${APP_NAME}-${VERSION}"

# ── Build ──
if command -v rpmbuild &>/dev/null; then
    rpmbuild --define "_topdir ${RPM_TOPDIR}" -bb "${RPM_TOPDIR}/SPECS/${APP_NAME}.spec"
    cp "${RPM_TOPDIR}"/RPMS/${ARCH}/*.rpm "${PACKAGE_DIR_RPM}/"
    echo " -> ${PACKAGE_DIR_RPM}/"
elif command -v docker &>/dev/null; then
    echo "rpmbuild not found locally, using Docker..."
    docker run --rm \
        -v "$(cd "${BUILD_DIR}" && pwd)":/build \
        fedora:latest \
        bash -c "
            dnf install -y rpm-build &&
            rpmbuild --define '_topdir /build/rpm-build' -bb /build/rpm-build/SPECS/${APP_NAME}.spec
        "
    RPM_FILE=$(find "${RPM_TOPDIR}/RPMS" -name "*.rpm" -print -quit 2>/dev/null)
    if [ -n "${RPM_FILE}" ]; then
        cp "${RPM_FILE}" "${PACKAGE_DIR_RPM}/"
        echo " -> ${PACKAGE_DIR_RPM}/$(basename "${RPM_FILE}")"
    else
        echo "ERROR: RPM file not found after build." >&2
        exit 1
    fi
else
    echo "ERROR: Neither rpmbuild nor Docker found." >&2
    echo "Install one of:" >&2
    echo "  brew install rpm" >&2
    echo "  OR apt install rpm" >&2
    echo "  OR install Docker: https://docs.docker.com/get-docker/" >&2
    exit 1
fi
