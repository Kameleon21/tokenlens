#!/usr/bin/env python3
"""Prepare semantic releases, validate metadata, and tag CI-approved main."""
import argparse
import datetime
import json
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
VERSION_PATTERN = r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)"


def version_tuple(value):
    if not re.fullmatch(VERSION_PATTERN, value):
        raise ValueError(f"Invalid semantic version: {value!r}; use MAJOR.MINOR.PATCH")
    return tuple(map(int, value.split(".")))


def current_version():
    text = (ROOT / "internal/app/version.go").read_text()
    match = re.search(r'^const Version = "([^"]+)"$', text, re.MULTILINE)
    if not match:
        raise ValueError("Cannot find the application Version constant")
    version_tuple(match[1])
    return match[1]


def next_version(current, bump):
    major, minor, patch = version_tuple(current)
    if bump == "patch":
        return f"{major}.{minor}.{patch + 1}"
    if bump == "minor":
        return f"{major}.{minor + 1}.0"
    if bump == "major":
        return f"{major + 1}.0.0"
    raise ValueError("Choose patch, minor, or major")


def release_notes(version):
    text = (ROOT / "CHANGELOG.md").read_text()
    match = re.search(r"^## \[([^]]+)\] - (\d{4}-\d{2}-\d{2})\n(.*?)(?=^## |\Z)", text, re.MULTILINE | re.DOTALL)
    if not match or match[1] != version:
        raise ValueError(f"The newest dated changelog section must be [{version}]")
    datetime.date.fromisoformat(match[2])
    # Exclude reference-link definitions at the end of a changelog.
    notes = re.split(r"^\[[^]]+\]:", match[3], maxsplit=1, flags=re.MULTILINE)[0].strip()
    if not re.search(r"^- ", notes, re.MULTILINE):
        raise ValueError("Release notes must contain at least one change")
    return notes


def check(tag=None):
    version = current_version()
    if tag is not None and tag != "v" + version:
        raise ValueError(f"Tag {tag!r} does not match application version v{version}")
    release_notes(version)
    return version


def prepare(bump):
    current = current_version()
    version = next_version(current, bump)
    changelog = ROOT / "CHANGELOG.md"
    text = changelog.read_text()
    match = re.search(r"^## Unreleased\n(.*?)(?=^## |\Z)", text, re.MULTILINE | re.DOTALL)
    if not match or not re.search(r"^- ", match[1], re.MULTILINE):
        raise ValueError("Add user-visible changes under Unreleased before preparing a release")
    if f"## [{version}]" in text:
        raise ValueError(f"Changelog already contains {version}")
    date = datetime.date.today().isoformat()
    replacement = f"## Unreleased\n\n## [{version}] - {date}\n\n{match[1].strip()}\n\n"
    updated = text[:match.start()] + replacement + text[match.end():]
    updated += f"\n[{version}]: https://github.com/Kameleon21/tokenlens/releases/tag/v{version}\n"
    source = ROOT / "internal/app/version.go"
    source.write_text(source.read_text().replace(f'const Version = "{current}"', f'const Version = "{version}"', 1))
    changelog.write_text(updated)
    print(f"Prepared v{version}. Review the changelog, run make release-snapshot, and open a release PR.")


def run(*args):
    return subprocess.check_output(args, cwd=ROOT, text=True).strip()


def git_remote(*args):
    # Use the existing gh login for HTTPS remotes without changing Git config.
    return run("git", "-c", "credential.helper=", "-c", "credential.helper=!gh auth git-credential", *args)


def require_passing_ci(runs):
    if not runs or runs[0]["status"] != "completed" or runs[0]["conclusion"] != "success":
        raise ValueError("The latest Checks workflow on this exact main commit must pass before tagging")


def tag_release():
    version = check()
    tag = "v" + version
    if run("git", "branch", "--show-current") != "main":
        raise ValueError("Switch to main before tagging a release")
    if run("git", "status", "--porcelain"):
        raise ValueError("Commit or preserve working-tree changes before tagging")
    git_remote("fetch", "origin", "main", "--tags")
    commit = run("git", "rev-parse", "HEAD")
    if commit != run("git", "rev-parse", "origin/main"):
        raise ValueError("Local main must equal origin/main before tagging")
    if run("git", "tag", "--list", tag):
        raise ValueError(f"Tag {tag} already exists; never move a published tag")
    runs = json.loads(run("gh", "run", "list", "--workflow", "ci.yml", "--branch", "main", "--event", "push", "--commit", commit, "--limit", "1", "--json", "status,conclusion"))
    require_passing_ci(runs)
    run("git", "tag", "-a", tag, commit, "-m", f"Tokenlens {tag}")
    git_remote("push", "origin", tag)
    print(f"Pushed {tag}. The Release workflow now builds and publishes via GoReleaser. Check gh run list --workflow release.yml.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("prepare").add_argument("bump", choices=["patch", "minor", "major"])
    commands.add_parser("check").add_argument("--tag")
    commands.add_parser("notes")
    commands.add_parser("tag")
    args = parser.parse_args()
    try:
        if args.command == "prepare":
            prepare(args.bump)
        elif args.command == "check":
            print(check(args.tag))
        elif args.command == "notes":
            print(release_notes(current_version()))
        else:
            tag_release()
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        parser.exit(1, f"Release stopped: {error}\n")


if __name__ == "__main__":
    main()
