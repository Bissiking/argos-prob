#!/usr/bin/env bash
set -euo pipefail

# build-msi.sh — Create a .msi installer for argos-prob (Windows)
# Usage: build-msi.sh <build_dir> <version>
#
# Requires: wixl from msitools (install via: brew install msitools)
#           or Wine + WiX Toolset for native MSI generation

BUILD_DIR="${1:?Usage: $0 <build_dir> <version>}"
VERSION="${2:?Usage: $0 <build_dir> <version>}"

APP_NAME="argos-prob"
PACKAGE_DIR="${BUILD_DIR}/packages"
MSI_DIR="${BUILD_DIR}/msi-staging"
WXLSRC="${MSI_DIR}/installer.wxs"

mkdir -p "${PACKAGE_DIR}"
rm -rf "${MSI_DIR}"
mkdir -p "${MSI_DIR}/bin"

# ── Copy binary ──
cp "${BUILD_DIR}/${APP_NAME}-windows-amd64.exe" "${MSI_DIR}/bin/${APP_NAME}.exe"

# ── Generate default config ──
cat > "${MSI_DIR}/bin/config.json" <<'CONF'
{
  "master_url": "https://argos-master.example.com",
  "listen_addr": ":9101",
  "mode": "passive",
  "interval": 60
}
CONF

# ── WiX source (wixl-compatible) ──
cat > "${WXLSRC}" <<WXS
<?xml version="1.0" encoding="UTF-8"?>
<Wix xmlns="http://schemas.microsoft.com/wix/2006/wi">
  <Product
    Id="*"
    Name="Argos Prob"
    Language="1033"
    Version="${VERSION}"
    Manufacturer="Argos Team"
    UpgradeCode="B1A2C3D4-E5F6-7890-ABCD-EF1234567890">

    <Package InstallerVersion="200"
             Compressed="yes"
             InstallScope="perMachine"
             Description="Argos Prob - Host monitoring agent"
             Manufacturer="Argos Team" />

    <MajorUpgrade DowngradeErrorMessage="A newer version is already installed." />

    <Media Id="1" Cabinet="argos-prob.cab" EmbedCab="yes" />

    <Feature Id="MainFeature" Title="Argos Prob" Level="1">
      <ComponentRef Id="MainBinary" />
      <ComponentRef Id="ConfigFile" />
    </Feature>

    <Directory Id="TARGETDIR" Name="SourceDir">
      <Directory Id="ProgramFilesFolder">
        <Directory Id="INSTALLFOLDER" Name="ArgosProb">
          <Component Id="MainBinary" Guid="A1B2C3D4-E5F6-7890-ABCD-EF1234567890">
            <File Id="${APP_NAME}.exe"
                  Source="${MSI_DIR}/bin/${APP_NAME}.exe"
                  KeyPath="yes" />
          </Component>

          <Component Id="ConfigFile" Guid="F0E1D2C3-B4A5-6789-0ABC-DEF123456789">
            <File Id="config.json"
                  Source="${MSI_DIR}/bin/config.json"
                  KeyPath="yes" />
          </Component>
        </Directory>
      </Directory>
    </Directory>

    <Icon Id="argos-prob.exe" SourceFile="${MSI_DIR}/bin/${APP_NAME}.exe" />
    <Property Id="ARPPRODUCTICON" Value="argos-prob.exe" />

  </Product>
</Wix>
WXS

# ── Build MSI ──
if command -v wixl &>/dev/null; then
    wixl -o "${PACKAGE_DIR}/${APP_NAME}-${VERSION}-windows-amd64.msi" "${WXLSRC}"
    echo " -> ${PACKAGE_DIR}/${APP_NAME}-${VERSION}-windows-amd64.msi"
else
    echo "ERROR: wixl not found." >&2
    echo "Install via: brew install msitools" >&2
    echo "Or use Wine + WiX: https://wixtoolset.org/" >&2
    exit 1
fi
