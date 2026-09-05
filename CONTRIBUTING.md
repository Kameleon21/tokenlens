# Contributing

Small fixes can go straight to a pull request. Open an issue before a large feature, new dependency, backend, or package restructuring so we can agree on scope.

## Setup and checks

Install Go 1.25 or newer. Clone the repository and run:

```sh
go run . --demo
go test ./...
go test -race ./...
go vet ./...
go build -o bin/tokenlens .
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

Maintainers review and merge changes. Passing CI is required by this contribution policy; repository branch protection is a separate GitHub setting.

## Project layout

Keep the single root Go package for now. `main.go` handles startup; `data.go` and `backend_*.go` integrate ccusage; `cache.go` stores reports; `dates.go` and `currency.go` handle dates and conversion; UI files render the dashboard; `export.go` writes exports. Tests live beside the code, and longer documentation belongs in `docs/`.

Extract an `internal/usage` package when a second data provider needs a shared model, and `internal/store` when persistent indexing has its own lifecycle. Avoid a public `pkg/` API until another project needs it. Keep the root executable so `go install github.com/Kameleon21/tokenlens@latest` continues to work.
