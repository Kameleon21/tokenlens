# Maintainer guide

## Why these files exist

| File | Purpose |
| --- | --- |
| `LICENSE` | Existing MIT terms for using and redistributing the project. |
| `CONTRIBUTING.md` | Sets contribution scope, checks, privacy expectations, and submission licensing. |
| `CODE_OF_CONDUCT.md` | Defines acceptable behavior, reporting, and moderation options. |
| `SECURITY.md` | Routes vulnerabilities privately and limits supported versions. |
| `SUPPORT.md` | Directs questions and sets realistic volunteer support expectations. |
| `GOVERNANCE.md` | Identifies who decides scope, access, and releases. |
| `.github/CODEOWNERS` | Requests the maintainer's review on incoming PRs. It does not grant access. |
| `.github/ISSUE_TEMPLATE/` | Collects actionable bug reports and feature proposals. |
| `.github/pull_request_template.md` | Reminds authors about validation, privacy, licensing, and documentation. |
| `.github/dependabot.yml` | Proposes weekly Go and Actions updates, grouping minor and patch updates with at most three open version-update PRs per ecosystem. Security updates have separate limits. |

These policies reduce ambiguity and maintenance work; they cannot prevent every
incident or replace consistent moderation and review.

## GitHub settings

The following settings were applied to `Kameleon21/tokenlens` alongside this
policy PR. They live in GitHub, not in these Markdown files; verify them after
repository transfers or changes to access and workflows.

- `main` requires pull requests, an up-to-date branch, resolved review threads,
  and successful GitHub Actions checks: `test`, `release-snapshot`,
  `preferences-platforms (macos-latest)`, and `preferences-platforms (windows-latest)`.
- The rules apply to administrators. Force pushes and deletion of `main` are blocked.
- Required approving reviews is zero while there is one maintainer: GitHub does
  not allow an author to approve their own PR. Contributors have no write access;
  the maintainer controls merging. CODEOWNERS requests review without introducing
  a self-approval deadlock. Add one required approval and code-owner review when
  a second trusted maintainer can review the owner's work.
- All external fork contributors require workflow approval. Inspect their diff,
  including workflow and build-script changes, before approving a run. Workflow
  approval permits execution; it is not approval to merge.
- Private vulnerability reporting, Dependabot alerts, and automated security
  fixes are enabled. Secret scanning and push protection were already enabled.
- Actions tokens default to read access and cannot approve PRs. The release job
  retains its explicit publishing permission. Automatic merged-branch deletion
  was already enabled.

Required checks are tied to the GitHub Actions app. Update branch protection if
check names change, or PRs can wait indefinitely for a nonexistent check.
Dependabot's weekly version-update configuration takes effect after it reaches
`main`; its configuration is not an automatic merge policy. It does not update
the manually pinned ccusage binaries, pricing snapshot, or GoReleaser version;
review those separately using the release guide.

## Routine review and triage

1. Check scope, reproduction details, and privacy. Redirect vulnerability reports
   to the private channel immediately; avoid quoting leaked information. Exposed
   credentials must be revoked by their owner; deleting a comment is insufficient.
2. Review code, dependencies, tests, licensing/attribution, and maintenance impact.
   Never run untrusted contribution scripts with local credentials. Keep fork
   testing on `pull_request` with read permissions; do not expose secrets through
   `pull_request_target` or privileged execution of contributor code.
3. Request focused revisions, or explain why a proposal is declined. Ask for
   missing information; close duplicates or issues that cannot proceed with an
   explanation, allowing reopening when useful information arrives. There is no
   automatic inactivity closure.
4. Merge only after review, required CI, and resolved discussions. Agent-assisted
   work still requires the owner's explicit merge confirmation. Follow
   [the release guide](releases.md) for publishing.
5. Keep access limited to trusted maintainers, use two-factor authentication, and
   review access when responsibilities change. After merging, clean up only
   merged branches and preserve uncommitted or unmerged work.

## Further reading

- [GitHub repository best practices](https://docs.github.com/en/repositories/creating-and-managing-repositories/best-practices-for-repositories)
- [Protected branches](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches)
- [Private vulnerability reporting](https://docs.github.com/en/code-security/how-tos/report-and-fix-vulnerabilities/configure-vulnerability-reporting/configure-for-a-repository)
- [Dependabot options](https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference)
