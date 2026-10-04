#!/usr/bin/env python3
"""Build Linux archives and Debian packages on Windows, Linux or macOS.

Requires Python 3 and Go. Debian members follow the documented ar/ustar format:
https://manpages.debian.org/bookworm/dpkg-dev/deb.5.en.html
"""
import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import tarfile

ROOT = Path(__file__).resolve().parent.parent
PACKAGING = ROOT / "packaging"


def tar_gzip(entries):
    archive = io.BytesIO()
    with tarfile.open(fileobj=archive, mode="w", format=tarfile.USTAR_FORMAT) as tar:
        for name, data, mode in entries:
            info = tarfile.TarInfo(name)
            info.mode = mode
            info.uid = info.gid = 0
            info.uname = info.gname = "root"
            info.mtime = 0
            if data is None:
                info.type = tarfile.DIRTYPE
                tar.addfile(info)
            else:
                info.size = len(data)
                tar.addfile(info, io.BytesIO(data))
    return gzip.compress(archive.getvalue(), mtime=0)


def write_deb(path, control, data):
    with path.open("wb") as output:
        output.write(b"!<arch>\n")
        for name, content in [("debian-binary", b"2.0\n"), ("control.tar.gz", control), ("data.tar.gz", data)]:
            header = f"{name + '/':<16}{0:<12}{0:<6}{0:<6}{'100644':<8}{len(content):<10}`\n"
            output.write(header.encode("ascii"))
            output.write(content)
            if len(content) % 2:
                output.write(b"\n")


def linux_package(binary, version, arch, destination):
    payload = binary.read_bytes()
    # Refuse an incorrectly cross-compiled binary before packaging it.
    machine = {"amd64": 62, "arm64": 183}[arch]
    if payload[:4] != b"\x7fELF" or payload[4:6] != b"\x02\x01" or int.from_bytes(payload[18:20], "little") != machine:
        raise ValueError(f"Invalid Linux {arch} binary: {binary}")
    service = (PACKAGING / "argos-prob.service").read_bytes().replace(b"\r\n", b"\n")
    control = (
        f"Package: argos-prob\nVersion: {version}\nSection: admin\nPriority: optional\n"
        f"Architecture: {arch}\nInstalled-Size: {(len(payload) + 1023) // 1024 + 16}\n"
        "Maintainer: Argos Team <dev@argos.example.com>\n"
        "Description: Argos Prob - Host monitoring agent\n"
        " Collects host metrics and controlled resource inventories for Argos.\n"
    ).encode()
    control_entries = [("./control", control, 0o644)]
    for name in ["preinst", "postinst", "prerm", "postrm"]:
        content = (PACKAGING / "debian" / name).read_bytes().replace(b"\r\n", b"\n")
        control_entries.append((f"./{name}", content, 0o755))
    data = tar_gzip([
        ("./usr/", None, 0o755), ("./usr/bin/", None, 0o755),
        ("./usr/bin/argos-prob", payload, 0o755),
        ("./etc/", None, 0o755), ("./etc/argos-prob/", None, 0o700),
        ("./lib/", None, 0o755), ("./lib/systemd/", None, 0o755),
        ("./lib/systemd/system/", None, 0o755),
        ("./lib/systemd/system/argos-prob.service", service, 0o644),
    ])
    package = destination / "packages" / f"argos-prob_{version}_{arch}.deb"
    package.parent.mkdir(parents=True, exist_ok=True)
    write_deb(package, tar_gzip(control_entries), data)
    archive = destination / f"argos-prob-{version}-linux-{arch}.tar.gz"
    archive.write_bytes(tar_gzip([(binary.name, payload, 0o755)]))
    return [package, archive]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--go", default="go", help="Go executable")
    parser.add_argument("--arch", choices=["amd64", "arm64", "all"], default="all")
    args = parser.parse_args()
    source = (ROOT / "internal/version/version.go").read_text(encoding="utf-8-sig")
    match = re.search(r'^var Version = "(\d+\.\d+\.\d+)"', source, re.MULTILINE)
    if not match:
        raise ValueError("Release version not found")
    version = match.group(1)
    destination = ROOT / "dist"
    destination.mkdir(exist_ok=True)
    artifacts = []
    for arch in (["amd64", "arm64"] if args.arch == "all" else [args.arch]):
        binary = destination / f"argos-prob-linux-{arch}"
        env = {**os.environ, "GOOS": "linux", "GOARCH": arch, "CGO_ENABLED": "0"}
        subprocess.run([
            args.go, "build", "-trimpath", "-buildvcs=false", "-ldflags",
            f"-s -w -X github.com/Bissiking/argos-prob/internal/version.Version={version}",
            "-o", str(binary), "./cmd/argos-prob",
        ], cwd=ROOT, env=env, check=True)
        for path in linux_package(binary, version, arch, destination):
            artifacts.append({"platform": "linux", "architecture": "x64" if arch == "amd64" else "arm64",
                "version": version, "packageType": "deb" if path.suffix == ".deb" else "tar.gz",
                "fileName": path.name, "path": path.relative_to(destination).as_posix(),
                "sha256": hashlib.sha256(path.read_bytes()).hexdigest()})
            print(path.relative_to(ROOT))
    manifest = destination / f"release-{version}-linux.json"
    manifest.write_text(json.dumps({"version": version, "artifacts": artifacts}, indent=2) + "\n", encoding="utf-8")
    sums = destination / f"SHA256SUMS-{version}-linux.txt"
    sums.write_text("".join(f"{item['sha256']}  {item['path']}\n" for item in artifacts), encoding="ascii")
    print(manifest.relative_to(ROOT))
    print(sums.relative_to(ROOT))


if __name__ == "__main__":
    main()
