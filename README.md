# Tokenlens

<p align="center">
  <img src="docs/assets/mascot-lensbot.png" width="220" alt="Tokenlens mascot: a mint lens robot inspecting a lavender token tile">
</p>

See your coding-agent token usage and estimated costs in your terminal. Compare agents and models, explore dates, and export reports.

![Tokenlens demo](docs/assets/demo.gif)

*The demo uses made-up data.*

## Install and run

You need **Go 1.25+** and either [Bun](https://bun.sh) or **ccusage 20.0.20**.

```sh
go install github.com/Kameleon21/tokenlens@latest
tokenlens
```

Make sure Go's binary directory (usually `~/go/bin`) is on your `PATH`.
From a local checkout, use `go install .` or `go run .`.
Tokenlens uses installed ccusage when available; otherwise Bun downloads and runs the pinned version automatically. The first download needs internet access.

Try it without any usage logs or backend:

```sh
tokenlens --demo
```

## Everyday use

```sh
tokenlens --currency EUR
tokenlens weekly --last 8
tokenlens --since 2026-08-01 --until 2026-08-31
tokenlens --theme light
```

The default date range is this calendar month. Dates are inclusive. The timezone is UTC unless you set `TZ` or `--timezone`.
Set `TOKENLENS_CURRENCY=EUR` in your shell to keep euros as your default.

| Key | What it does |
| --- | --- |
| `1`–`5` / `tab` | Switch views |
| `d` / `w` / `m` | Group by day, week, or month |
| `a` / `f` / `x` | Filter agent, filter model, clear filters |
| `t` / `p` | Enter dates / cycle date presets |
| `c` / `e` | Switch cost/token view / currency |
| `r` | Fetch a fresh report |
| `o` | Export JSON, CSV, SVG, or PNG |
| `?` / `q` | All controls / quit |

## Loading speed

Recent reports are reused for **five minutes**, so reopening Tokenlens or returning to the same dates can skip ccusage. Older saved reports stay visible while updated data loads. The report timestamp and cached label show what you are viewing.

- Press `r` to fetch new usage immediately. Pressing it again during the same load keeps that request running.
- Currency, grouping, and filter changes do not rerun the usage report.
- Use `--cache-ttl 30s` for a shorter reuse window, or `--cache-ttl 0` to always reload.
- Use `--offline` for ccusage's cached pricing mode. It can be faster, but cost estimates may differ. Currency conversion still needs an exchange-rate request.
- `--no-cache` disables report caching. Saved reports live in your OS user cache under `tokenlens`; `--cache-dir` changes that location.

First loads and dates without a saved report still wait for ccusage. See the [performance plan](docs/performance.md) for how Tokenlens could eventually read usage logs itself.

## What the numbers mean

Costs are estimates from ccusage, not your subscription bill. Currency conversion uses the latest published ECB reference rate via Frankfurter, including for historical usage. Missing values are shown as unavailable.

Usage reports stay local. Non-USD conversion sends only the currency request. Cached reports and exports can contain private project or session names. Demo mode uses synthetic data and makes no backend or exchange-rate requests.

See the [usage reference](docs/usage.md) for billing comparisons, exports, cache details, and data limits. Run `tokenlens --help` for all flags.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) for setup, checks, and pull request guidelines. Small fixes and documentation improvements are welcome.

## License

[MIT](LICENSE).
