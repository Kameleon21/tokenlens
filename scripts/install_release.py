#!/usr/bin/env python3
"""Install a complete official Tokenlens release on macOS/Linux, verifying SHA-256."""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path, PurePosixPath
import platform
import re
import shutil
import subprocess
import tarfile
import tempfile
import urllib.request

REPO = 'Kameleon21/tokenlens'
MAX_DOWNLOAD = 128 * 1024 * 1024


def download(url, limit=MAX_DOWNLOAD):
    if not url.startswith('https://'):
        raise ValueError('Downloads require HTTPS')
    req = urllib.request.Request(url, headers={'User-Agent': 'tokenlens-updater', 'Accept': 'application/vnd.github+json'})
    with urllib.request.urlopen(req, timeout=60) as response:
        data = response.read(limit + 1)
    if len(data) > limit:
        raise ValueError('Download exceeds size limit')
    return data


def release_assets(release, target):
    tag = release.get('tag_name', '')
    if not re.fullmatch(r'v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)', tag) or release.get('draft') or release.get('prerelease'):
        raise ValueError('Expected a published stable semantic release')
    archive = f'tokenlens_{tag[1:]}_{target}.tar.gz'
    checksum = f'tokenlens_{tag[1:]}_checksums.txt'
    assets = {}
    for name in (archive, checksum):
        matches = [a for a in release.get('assets', []) if a.get('name') == name]
        expected = f'https://github.com/{REPO}/releases/download/{tag}/{name}'
        if len(matches) != 1 or matches[0].get('browser_download_url') != expected:
            raise ValueError(f'Missing or unexpected official asset: {name}')
        assets[name] = expected
    return tag, archive, checksum, assets


def verify_checksum(archive, name, checksums):
    matches = []
    for line in checksums.decode('utf-8').splitlines():
        parts = line.split()
        if len(parts) == 2 and parts[1].lstrip('*') == name:
            matches.append(parts[0])
    if len(matches) != 1 or not re.fullmatch('[0-9a-fA-F]{64}', matches[0]):
        raise ValueError('Missing or ambiguous SHA-256 checksum')
    digest = hashlib.sha256(archive).hexdigest()
    if digest != matches[0].lower():
        raise ValueError('SHA-256 mismatch; existing installation was not changed')
    return digest


def extract_archive(data, destination):
    total = 0
    names = set()
    with tarfile.open(fileobj=io.BytesIO(data), mode='r:gz') as archive:
        for member in archive:
            path = PurePosixPath(member.name)
            if path.is_absolute() or '..' in path.parts or '\\' in member.name or path.as_posix() in names:
                raise ValueError('Unsafe or duplicate archive path')
            names.add(path.as_posix())
            if len(names) > 10000:
                raise ValueError("Too many archive members")
            if not member.isfile() and not member.isdir():
                raise ValueError('Archive links and special files are not permitted')
            total += member.size
            if total > 256 * 1024 * 1024:
                raise ValueError('Unpacked release exceeds size limit')
            target = destination.joinpath(*path.parts)
            if member.isdir():
                target.mkdir(parents=True, exist_ok=True)
            else:
                target.parent.mkdir(parents=True, exist_ok=True)
                with archive.extractfile(member) as source, target.open('wb') as output:
                    shutil.copyfileobj(source, output)
                target.chmod(0o755 if member.mode & 0o111 else 0o644)
    if not (destination / 'tokenlens').is_file():
        raise ValueError('Release has no Tokenlens executable')


def install_verified(data, tag, target, digest, data_dir, bin_dir):
    releases = data_dir / 'releases'
    releases.mkdir(parents=True, exist_ok=True)
    bin_dir.mkdir(parents=True, exist_ok=True)
    # Stage a unique immutable directory. Only switch the executable symlink after
    # extraction and version verification; an interrupted update keeps the old app.
    with tempfile.TemporaryDirectory(prefix='.staging-', dir=releases) as temporary:
        staging = Path(temporary)
        extract_archive(data, staging)
        binary = staging / 'tokenlens'
        binary.chmod(0o755)
        version = subprocess.check_output([str(binary), '--version'], text=True, timeout=10).strip()
        if version != 'tokenlens ' + tag[1:]:
            raise ValueError('Downloaded binary version does not match the release')
        if (staging / 'libexec').exists() and not (staging / 'libexec/ccusage').is_file():
            raise ValueError('Release bundle has no native ccusage companion')
        final = releases / f'{tag}-{target}-{digest[:16]}'
        if final.exists():
            # Do not trust or overwrite a previously modified installation.
            final = releases / (final.name + '-' + staging.name.removeprefix('.staging-'))
        staging.rename(final)
    link = bin_dir / 'tokenlens'
    if link.exists() and not link.is_symlink():
        backup = bin_dir / ('tokenlens-before-release-installer-' + digest[:12])
        if backup.exists():
            raise ValueError(f'Backup already exists: {backup}; installation left untouched')
        shutil.copy2(link, backup)
    replacement = bin_dir / ('.tokenlens-' + final.name)
    try:
        replacement.symlink_to(final / 'tokenlens')
        os.replace(replacement, link)
    finally:
        replacement.unlink(missing_ok=True)
    # Future bundled releases carry their updater too. Switch that entry point
    # after the app succeeds; a running Python process keeps its loaded code.
    updater = final / 'scripts/install_release.py'
    if updater.is_file():
        replacement = data_dir / ('.update-' + final.name)
        try:
            replacement.symlink_to(updater)
            os.replace(replacement, data_dir / 'update.py')
        except OSError as error:
            print(f'App updated, but updater refresh failed: {error}')
        finally:
            replacement.unlink(missing_ok=True)
    return link


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--version', help='Specific stable tag, e.g. v0.3.0 (default latest release)')
    parser.add_argument('--allow-downgrade', action='store_true', help='Explicitly allow replacing a newer installed version')
    parser.add_argument('--bin-dir', type=Path, default=Path.home() / '.local/bin')
    parser.add_argument('--data-dir', type=Path, default=Path.home() / '.local/share/tokenlens')
    args = parser.parse_args()
    system = {'Darwin': 'darwin', 'Linux': 'linux'}.get(platform.system())
    arch = {'arm64': 'arm64', 'aarch64': 'arm64', 'x86_64': 'amd64', 'AMD64': 'amd64'}.get(platform.machine())
    if not system or not arch:
        parser.error('This installer supports macOS/Linux amd64/arm64. On Windows extract the zip and keep libexec beside tokenlens.exe.')
    if args.version and not re.fullmatch(r'v[0-9]+\.[0-9]+\.[0-9]+', args.version):
        parser.error('Use a stable version tag such as v0.3.0')
    endpoint = 'tags/' + args.version if args.version else 'latest'
    try:
        release = json.loads(download(f'https://api.github.com/repos/{REPO}/releases/{endpoint}', 2 * 1024 * 1024))
        target = system + '_' + arch
        tag, name, checksum, assets = release_assets(release, target)
        installed_path = args.bin_dir.expanduser() / 'tokenlens'
        if installed_path.exists() and not args.allow_downgrade:
            existing = subprocess.check_output([str(installed_path), '--version'], text=True, timeout=10)
            match = re.fullmatch(r'tokenlens ([0-9]+)\.([0-9]+)\.([0-9]+)\s*', existing)
            if match and tuple(map(int, match.groups())) > tuple(map(int, tag[1:].split('.'))):
                raise ValueError(f'Installed version is newer than published {tag}; use --allow-downgrade only if intentional')
        print(f'Downloading Tokenlens {tag} for {target}...', flush=True)
        data = download(assets[name])
        digest = verify_checksum(data, name, download(assets[checksum], 1024 * 1024))
        installed = install_verified(data, tag, target, digest, args.data_dir.expanduser().absolute(), args.bin_dir.expanduser().absolute())
        print(f'Installed {tag}; SHA-256 verified. Executable: {installed}')
        print(f'Keep {installed.parent} on PATH. Preferences and usage caches are preserved.')
    except (OSError, ValueError, subprocess.SubprocessError, tarfile.TarError) as error:
        parser.exit(1, f'Update stopped: {error}\n')


if __name__ == '__main__':
    main()
