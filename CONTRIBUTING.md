# Contributing

Small fixes can go straight to a pull request. Open an issue before a large feature, new dependency, backend, or package restructuring so we can agree on scope.

## Before you contribute

Read the [code of conduct](CODE_OF_CONDUCT.md). For questions and troubleshooting,
see [SUPPORT.md](SUPPORT.md); report vulnerabilities privately as described in
[SECURITY.md](SECURITY.md).

Search existing issues and pull requests first. For code changes, fork the
repository, create a topic branch from `main`, and open a PR back to `main`.
Draft PRs are welcome when you want early feedback. An issue or proposal is not
a commitment that a feature will be implemented or a PR accepted.

## Contribution rights

By submitting a contribution, you agree that it is provided under this project's
[MIT license](LICENSE). Submit only work you have the right to contribute and
preserve third-party license notices and attribution. No separate contributor
license agreement or commit sign-off is required. If you use AI tools, you remain
responsible for understanding, checking, and having the rights to the submitted
code; generated changes receive the same review as other contributions.

## Setup and checks

Install Go 1.25 or newer, Python 3, and Make. Clone the repository and run:

```sh
go run . --demo
make check
```

Run `gofmt` on Go files you change. Normal tests use synthetic fixtures and local HTTP test servers; they must not require personal logs, credentials, Bun, or an external service. Live integration tests are opt-in.

## Pull requests

- Keep each PR focused. Describe the problem, the resulting behavior, and how you checked it.
- Add regression coverage for behavior changes, especially dates, caching, cancellation, and missing data.
- Use synthetic data in fixtures, screenshots, and exports. Never commit personal logs, reports, cache files, or credentials.
- Preserve unknown and partial metrics. Do not replace missing data with zero or invent cost, repository, or time attribution.
- Keep external work asynchronous and cancellable. Show cached status and timestamps accurately.
- Update user documentation when controls or flags change. For visual changes, include a demo screenshot and check a compact terminal too.
- Be respectful and constructive in issues and reviews.

After a PR is merged, delete its remote branch and remove the local branch once its commits are confirmed on `main`. Keep unmerged branches and any uncommitted work.

Maintainers review and merge changes. All required CI checks must pass and review
conversations must be resolved before merging. External fork workflows need a
maintainer's approval to run; a waiting workflow does not mean your PR is rejected.
Maintainers may request revisions or decline changes based on scope, complexity,
compatibility, or ongoing maintenance cost. See [GOVERNANCE.md](GOVERNANCE.md) for
decisions and [the maintainer guide](docs/maintaining.md) for repository safeguards.

## Project layout

```text
main.go                 Small executable entry point; preserves go install
internal/
  app/                  Startup, ccusage adapter, caching, terminal UI, exports
    *_test.go           App and backend-flow tests beside the implementation
  datefilter/           Date parsing and calendar ranges
    range_test.go       Date, timezone, and DST tests
docs/
  assets/               README mascot, screenshots, and demo media
  usage.md              Detailed user guide
  performance.md        Loading behavior and future native indexing plan
.github/
  workflows/            Automated checks
  pull_request_template.md
```

Keep tests beside their packages, as is conventional in Go. Use a package's `testdata/` directory if file-based fixtures are needed. The date package has no UI or backend dependencies. The app package owns orchestration and presentation; split providers or persistent indexing into additional internal packages when they gain an independent lifecycle. Avoid a public `pkg/` API until another project needs it.

## Versioning and releases

Use semantic versions starting at `0.1.0`. For every user-visible change, add a
concise entry under **Unreleased** in [CHANGELOG.md](CHANGELOG.md). Release PRs
update `Version` in `internal/app/version.go` and move those entries into a
versioned changelog section. Follow [docs/releases.md](docs/releases.md) for bump
rules, `make release-prepare`, required checks, and tag-triggered publishing with GoReleaser.
