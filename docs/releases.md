# Versioning and releases

Tokenlens uses semantic versions (`MAJOR.MINOR.PATCH`) and Git tags with a `v`
prefix, starting at `v0.1.0`. The application version and release notes are recorded in
`internal/app/version.go` and `CHANGELOG.md`. Published versions are listed on
[GitHub Releases](https://github.com/Kameleon21/tokenlens/releases).

## Choosing a version

- **Patch**, such as `0.1.1`: compatible bug fixes.
- **Minor**, such as `0.2.0`: new features. While the major version is zero,
  breaking changes also require a minor bump and explicit migration notes.
- **Major**, such as `1.0.0`: the first stable interface commitment; afterward,
  incompatible changes require a major bump.
- Documentation-only changes normally remain under Unreleased until the next
  application release.

Treat documented CLI flags, configuration, and exported data formats as public
interfaces. Never reuse or move a published version tag. Fix a bad release with
a new version.

## During development

`internal/app/version.go` is the single source of the application version. The
`--version` flag prints it for both local builds and `go install` builds; no
special linker flags are required. An untagged checkout can include additional
changes beyond that version, so use the Git commit when reporting development
builds.

Add user-visible changes to the Unreleased section of `CHANGELOG.md` in each PR.
Keep entries concise, describe resulting behavior, and call out breaking changes.
Do not bump the version for every commit.

## Preparing a release

1. Create a release PR that sets `Version` in `internal/app/version.go` to the
   chosen number. Move Unreleased entries under a heading such as
   `## [0.1.0] - YYYY-MM-DD`, using the actual release date, and leave an empty
   Unreleased section for future work.
2. Run `go test -race ./...`, `go vet ./...`, and `go build -o bin/tokenlens .`.
   Verify `./bin/tokenlens --version` matches the intended version and try
   `./bin/tokenlens --demo`. Check formatting and wait for GitHub CI to pass.
3. Obtain explicit maintainer approval to merge. After merging, update local
   `main`, confirm the working tree is clean, and confirm the intended commit's
   GitHub checks passed. Publish only the approved release commit.
4. Copy that version's changelog entries into a release notes file. Create and
   push an annotated tag, then publish using the terminal's `gh` CLI. For the
   first release, after replacing the example commit with the verified full SHA:

   ```sh
   git tag -a v0.1.0 APPROVED_COMMIT_SHA -m "Tokenlens v0.1.0"
   git -c credential.helper= -c 'credential.helper=!gh auth git-credential' push origin v0.1.0
   gh release create v0.1.0 --verify-tag --title "Tokenlens v0.1.0" --notes-file /tmp/tokenlens-release-notes.md
   ```

   Adapt the version and notes path for subsequent releases. `--verify-tag`
   prevents accidentally creating a release against a new, unintended tag.
5. Verify `gh release view v0.1.0` and the tag-triggered checks using `gh run list`.
   Check installation of the exact release:

   ```sh
   go install github.com/Kameleon21/tokenlens@v0.1.0
   tokenlens --version
   ```

The initial distribution uses Go installation and GitHub's source archives.
There is no automatic release publishing or prebuilt binary upload. A merged
feature PR by itself does not create a release.
