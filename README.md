# Tokenlens

A calm, keyboard-driven terminal dashboard for your coding-agent usage. Built in Go with Bubble Tea, Lip Gloss, and Charm components. Local data, exact numbers, one snapshot.

![Tokenlens overview](docs/overview.png)

## Run

Requires **Go 1.25+** to build and a locally installed **ccusage with unified `--sections` and `--by-agent` support**. Verified against the released **ccusage 20.0.20** native executable, including its actual JSON output. Older Claude-only releases are not supported. Tokenlens is a native Go application; it does not run npm/npx/bunx at runtime.

Install the backend following [ccusage installation](https://ccusage.com/guide/installation). One available method is `npm install -g ccusage@20.0.20` (Node/npm are needed for that installation method, including its optional native platform dependency). Verify with `ccusage --version` and `ccusage daily --help`.

```sh
go build -o bin/tokenlens .
./bin/tokenlens --demo                  # synthetic data; no ccusage required
./bin/tokenlens                         # current calendar month, overview
./bin/tokenlens weekly                  # same range, initially weekly
./bin/tokenlens monthly --last 3        # current and previous two months
./bin/tokenlens daily --last 7 --timezone Europe/Dublin
./bin/tokenlens --since 20260901 --until 2026-09-30
./bin/tokenlens --since 20260101         # open upper bound; no month default
./bin/tokenlens --ccusage /path/to/ccusage
```

Optionally put the binary on your PATH. `tokenlens --help` lists options. The default timezone is **UTC**, or the `TZ` environment variable when set. Use `--timezone` explicitly for local calendar boundaries. IANA timezone data is embedded in the binary.

## Explore

- **Overview:** persistent token and estimated USD totals, ranked period activity, biggest agent cost contributor.
- **Agents / Models:** independent dynamic rankings, including any source or model returned by ccusage.
- **Tokens / cache:** input, output, cache read, and cache write counts.
- **Sessions:** ranked sessions with Enter details and backend metadata.

| Key | Action |
| --- | --- |
| `1`–`5`, `tab` / `shift+tab` | Switch view |
| `d` / `w` / `m` | Daily / weekly / monthly grouping |
| `c` | Estimated cost / token metric |
| `s` | Sort largest, smallest, or name |
| `↑` / `↓`, `j` / `k` | Select row and scroll |
| `home` / `end`, `g` / `G` | First / last row |
| `enter` | Open / close selected row details |
| `a` | Cycle agent filter |
| `f` | Cycle model filter independently |
| `x` | Clear both filters |
| `t` | Edit range: `2026-09-01 2026-09-30`, `* 2026-09-30`, `month`, or `last 7` |
| `r` | Refresh the snapshot |
| `?` | Help |
| `esc` | Close editor, help, error, or details |
| `q` / `ctrl+c` | Quit |

Dates accept valid `YYYYMMDD` or `YYYY-MM-DD` and are inclusive. No range flags means the entire current calendar month. An explicit single bound remains open on the other side. `--last N` includes today / this week / this month, ends today, and cannot combine with explicit date bounds. Weeks start Monday. Switching grouping keeps the selected date range; use `t` → `last N` to resolve another relative range with the new grouping.

Use a terminal of at least **50 × 16**; **100 × 32** or larger gives the clearest presentation. The UI adapts its tabs, chart widths, and visible row count. Agent colors use a stable palette hash. Exact counts, USD values to four decimals, and percentages accompany the bars. Percentages use the visible ranking total; partial totals suppress misleading percentages.

## Backend and honesty

Tokenlens executes one structured command per load:

```sh
ccusage daily --sections daily,weekly,monthly,session --by-agent --json \
  --timezone UTC --since 2026-09-01 --until 2026-09-30
```

Daily, weekly, monthly, and session reports stay in memory, so views and filters change without another subprocess. Range changes reload; refresh is explicit. Loads run asynchronously, cancel superseded requests, and time out after two minutes. Failed refreshes preserve the previous snapshot and its range. The header shows snapshot time. No log parsers, telemetry, background synchronization, or runtime package installation are included.

Costs are **estimates in USD**, provided by ccusage. They are not subscription balance, money remaining, or revenue. Backend pricing/cache/network behavior follows your ccusage configuration. This is a local viewer, not an offline guarantee for ccusage itself.

Missing JSON metrics display **unavailable**; incomplete aggregates display **`+ ?`**. Reported zero stays zero: Tokenlens cannot infer whether an upstream source used zero as an unavailable placeholder. Model token totals are derived only when all four token categories exist, using ccusage's normalized input/output/cache schema. Category-level dollar estimates are unavailable, so Tokens / cache always shows token counts. Session range attribution follows ccusage's session report semantics. Missing model/agent breakdowns cannot be reconstructed; unavailable breakdowns may produce an empty filtered view. Upstream schema changes produce a helpful error instead of silently treating an older single-source report as complete.

Demo data is synthetic and deterministic for a given range, bounded to 3,660 days to avoid huge fixtures. It is labeled **DEMO · SYNTHETIC** throughout. Screenshots use demo data only.

## Development

```sh
go test ./...
go test -race ./...
go vet ./...
go build -o bin/tokenlens .
```

Tests cover invalid/leap dates, month defaults, open bounds, Monday weeks, timezone/DST boundaries, backend command construction, dynamic sources/models, intersections, missing metrics, aggregation, stale load protection, navigation, and terminal dimensions. Optional real-backend integration: save a multi-section JSON report outside the repository and run `TOKENLENS_TEST_SNAPSHOT=/path/to/report.json go test -run TestRealSnapshotOptIn -v`. Never commit personal usage logs.

Implementation is intentionally small: `dates.go` resolves calendar bounds, `data.go` adapts ccusage JSON, `ui.go` handles the Charm UI, and `demo.go` generates review data. See [ccusage CLI options](https://ccusage.com/guide/cli-options) and [JSON schema examples](https://ccusage.com/guide/json-output).

## License

MIT. ccusage and Charm dependencies retain their respective licenses.
