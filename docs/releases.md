# Versioning and releases

Tokenlens uses semantic versions (`MAJOR.MINOR.PATCH`) and immutable `v`-prefixed
Git tags. `internal/app/version.go` is the single source of the application
version, including for `go install` builds. The tag and newest dated section in
`CHANGELOG.md` must match it. Untagged development builds can contain additional
changes; include the Git commit when reporting them.

## Choosing a version

- **Patch**: compatible bug fixes (`0.2.0` → `0.2.1`).
- **Minor**: new features (`0.2.0` → `0.3.0`). Before 1.0, breaking changes also
  require a minor bump and explicit migration notes.
- **Major**: the first stable interface commitment (`1.0.0`); afterward,
  incompatible changes require a major bump.

Documented CLI flags, configuration, and exported data formats are public
interfaces. Documentation-only changes normally wait until the next application
release. This process currently supports stable versions only, not prereleases.
Never reuse or move a published tag; fix a bad release with a new version.

## During development

Add user-visible changes under **Unreleased** in `CHANGELOG.md` in each PR. Do
not bump the version for every commit. Run `make check` (Go 1.25+, Python 3, and
Make required). CI also validates GoReleaser and builds all six release archives
without publishing, including on pull requests.

## Prepare and review

1. Start a release branch from current `main`.
2. Run `make release-prepare BUMP=patch` (or `minor` / `major`). This bumps the
   application version, moves Unreleased notes into a dated release section,
   and leaves an empty Unreleased section. Review the notes and migration advice.
3. Install the GoReleaser version recorded in `.goreleaser-version`, using the
   [official installation instructions](https://goreleaser.com/install/).
   Run `make release-snapshot`. It runs checks and creates archives and SHA-256
   checksums under `dist/` without creating a tag or publishing a release.
   Snapshots have a development archive version; the binary retains the source
   version. Check the native binary with `--version` and try `--demo`.
4. Commit, push, and open a release PR. Wait for both Checks jobs to pass and
   obtain explicit maintainer approval before merging.

## Publish

After the approved release PR is merged:

```sh
git switch main
git pull --ff-only
make release-tag
```

The tagging command requires Git and an authenticated `gh` CLI with repository
write access. It verifies a clean local `main`, fetches remote main and tags,
requires local main to equal remote main, rejects existing tags, and checks that
the latest Checks run on that exact commit passed. It then creates and pushes an
annotated tag. Running this command is the explicit publishing step; merging a
PR alone does not publish anything.

The tag-triggered Release workflow validates the tag, changelog, and main-branch
ancestry, reruns project checks, then invokes the pinned GoReleaser version to:

- Build macOS, Linux, and Windows binaries for amd64 and arm64 without CGO.
- Package tar.gz archives (zip on Windows), with native ccusage, licenses, documentation, and the macOS/Linux updater.
- Generate SHA-256 checksums and publish assets to GitHub Releases.
- Use the version's changelog entries as release notes, with install instructions.

Only the publishing job receives `contents: write`; it uses GitHub's built-in
`GITHUB_TOKEN`. No additional secret or Homebrew tap is required. The setup
follows [Oku's release workflow](https://github.com/Kameleon21/oku/blob/develop/.github/workflows/release.yml)
and [build configuration](https://github.com/Kameleon21/oku/blob/develop/.goreleaser.yaml),
with Tokenlens's curated changelog and version checks.

## Verify or recover

```sh
gh run list --workflow release.yml
gh release view v0.2.0
go install github.com/Kameleon21/tokenlens@v0.2.0
tokenlens --version
```

Replace the example version with the version just tagged. Wait for the Release
workflow to succeed and verify six platform archives plus the checksum file.
Download and extract your platform's archive, verify its checksum, and run
`tokenlens --version` and `tokenlens --demo`. Release bundles need neither Go nor Bun; keep the `libexec` companion directory
beside the executable. Go installations still need an external backend.

If the tag push fails, inspect local and remote tags before retrying: the local
annotated tag may already exist. If publishing fails, inspect the workflow logs
and any partial GitHub release before rerunning the failed job. Never move the
tag. If code or metadata must change, prepare a new patch release instead.

## Installing and updating

On macOS/Linux, download and extract an official release archive, then run:

```sh
python3 scripts/install_release.py
# To retain a previous Go binary location:
python3 scripts/install_release.py --bin-dir "$HOME/go/bin"
# Select a particular stable release:
python3 scripts/install_release.py --version v0.3.0
```

The Python 3 updater reads the latest published stable GitHub release, verifies
the archive's SHA-256 checksum, extracts the full package into a unique version
directory under `~/.local/share/tokenlens/releases`, validates `--version`, and
atomically switches the `tokenlens` symlink in the selected bin directory.
It preserves existing preferences/caches and keeps older installations. An old
standalone executable is backed up before the symlink replaces it. Downloads,
checksum failures, unsafe archive paths/links, or version mismatches never switch
the installed executable. Downgrades require `--allow-downgrade`.

A shell convenience function can call a saved copy of this updater:

```sh
tokenlens-update() {
  python3 "$HOME/.local/share/tokenlens/update.py" --bin-dir "$HOME/go/bin" "$@"
}
```

Save `scripts/install_release.py` as `~/.local/share/tokenlens/update.py` first.
Bundled releases also refresh the saved `update.py` entry point automatically.
The updater supports older binary-only releases as well as complete bundles;
only releases containing `libexec` provide the bundled-backend improvement.
On Windows, extract the complete zip and add its directory to PATH. If native
system timezone discovery cannot find an IANA name, supply `--timezone`, for
example `--timezone Europe/Dublin`.

## Maintaining bundled dependencies

`third_party/ccusage-lock.json` pins all six npm platform packages and their
SHA-512 integrity values. The GoReleaser before hook downloads and verifies them
without running npm scripts, stages only native executables, and includes the
license notices. Update the lock in a reviewed PR when changing ccusage version.

`make prices-update` refreshes the committed initial catalog. Review and commit
its diff before release; ordinary release builds use the committed snapshot and
do not silently change prices. The runtime independently refreshes the public
LiteLLM catalog. Tests cover rate validation, missing coverage, long-context and
fast-rate fields, cache fallback, and refresh coordination.
