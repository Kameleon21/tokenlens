#!/usr/bin/env python3
"""Check all six generated release bundles and smoke-test the native executables."""
import hashlib
import json
from pathlib import Path
import platform
import subprocess
import tarfile
import tempfile
import zipfile

from install_release import extract_archive
from release import current_version


def main():
    root = Path(__file__).resolve().parents[1]
    directory = root / 'dist'
    backend_version = json.loads((root / 'third_party/ccusage-lock.json').read_text())['version']
    checksum_files = list(directory.glob('*checksums.txt'))
    if len(checksum_files) != 1:
        raise ValueError('Expected exactly one checksum file')
    names = []
    for line in checksum_files[0].read_text().splitlines():
        digest, name = line.split()
        if Path(name).name != name:
            raise ValueError('Unexpected checksum path')
        if hashlib.sha256((directory / name).read_bytes()).hexdigest() != digest:
            raise ValueError('Archive checksum mismatch')
        names.append(name)
    if len(names) != 6 or len(set(names)) != 6:
        raise ValueError('Expected six unique platform archives')
    native_os = {'Darwin':'darwin','Linux':'linux','Windows':'windows'}.get(platform.system())
    native_arch = {'arm64':'arm64','aarch64':'arm64','x86_64':'amd64','AMD64':'amd64'}.get(platform.machine())
    for system in ['darwin','linux','windows']:
        for arch in ['amd64','arm64']:
            extension = '.zip' if system == 'windows' else '.tar.gz'
            matches = [name for name in names if name.endswith(f'_{system}_{arch}{extension}')]
            if len(matches) != 1:
                raise ValueError(f'Missing archive for {system}/{arch}')
            path = directory / matches[0]
            if system == 'windows':
                with zipfile.ZipFile(path) as archive:
                    members = archive.namelist()
            else:
                with tarfile.open(path) as archive:
                    members = archive.getnames()
                    for binary in ['tokenlens','libexec/ccusage']:
                        if not archive.getmember(binary).mode & 0o111:
                            raise ValueError(f'{binary} is not executable')
            suffix = '.exe' if system == 'windows' else ''
            required = ['tokenlens'+suffix,'libexec/ccusage'+suffix,'scripts/install_release.py','third_party/ccusage-LICENSE','third_party/litellm-LICENSE']
            if not all(name in members for name in required):
                raise ValueError(f'Incomplete bundle: {path.name}')
            if system == native_os and arch == native_arch and system != 'windows':
                with tempfile.TemporaryDirectory(prefix='tokenlens-bundle-check-') as temporary:
                    root = Path(temporary)
                    extract_archive(path.read_bytes(), root)
                    expected = {'tokenlens':'tokenlens '+current_version(), 'libexec/ccusage':'ccusage '+backend_version}
                    for binary, version in expected.items():
                        actual = subprocess.check_output([str(root/binary),'--version'],text=True,timeout=10).strip()
                        if actual != version:
                            raise ValueError(f'Unexpected version for {binary}: {actual}')
            print(f'Verified {system}/{arch} bundle')


if __name__ == '__main__':
    main()
