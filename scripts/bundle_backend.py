#!/usr/bin/env python3
"""Stage pinned, integrity-verified native ccusage binaries for GoReleaser."""
import base64
import hashlib
import io
import json
from pathlib import Path
import tarfile
import urllib.request

ROOT = Path(__file__).resolve().parents[1]


def verified_binary(archive, integrity, filename):
    algorithm, digest = integrity.split('-', 1)
    if algorithm != 'sha512' or hashlib.sha512(archive).digest() != base64.b64decode(digest, validate=True):
        raise ValueError('ccusage package integrity mismatch')
    with tarfile.open(fileobj=io.BytesIO(archive), mode='r:gz') as package:
        members = [m for m in package.getmembers() if m.name == 'package/bin/' + filename]
        if len(members) != 1 or not members[0].isfile() or members[0].size > 64 * 1024 * 1024:
            raise ValueError('Missing or invalid native binary')
        return package.extractfile(members[0]).read()


def main():
    lock = json.loads((ROOT / 'third_party/ccusage-lock.json').read_text())
    for target, package in lock['targets'].items():
        if not package['url'].startswith('https://registry.npmjs.org/@ccusage/'):
            raise ValueError('Unexpected package origin')
        with urllib.request.urlopen(package['url'], timeout=60) as response:
            archive = response.read(32 * 1024 * 1024 + 1)
        if len(archive) > 32 * 1024 * 1024:
            raise ValueError('Package exceeds size limit')
        filename = 'ccusage.exe' if target.startswith('windows_') else 'ccusage'
        data = verified_binary(archive, package['integrity'], filename)
        directory = ROOT / '.release-backends' / target
        directory.mkdir(parents=True, exist_ok=True)
        binary = directory / filename
        binary.write_bytes(data)
        binary.chmod(0o755)
        print(f'Verified ccusage {lock["version"]}: {target}')


if __name__ == '__main__':
    main()
