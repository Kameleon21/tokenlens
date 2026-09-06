# Changelog

User-visible changes are recorded here. Versions follow the conventions in
[the release guide](docs/releases.md).

## Unreleased

### Fixed

- Keep help within the selected tab and restore its underlying detail view and selection when closing with Escape.

## [0.5.0] - 2026-09-06

### Added

- Persistent `export_dir` in TOML preferences, with home-directory expansion and per-run `--export-dir` overrides.
- `tokenlens doctor` for local configuration, path access, timezone, backend discovery, and pricing diagnostics with actionable messages and exit codes.

## [0.4.1] - 2026-09-06

### Fixed

- Apply the saved date format across all tabs, overview charts, activity details, range editing, exchange-rate labels, and SVG/PNG exports.

## [0.4.0] - 2026-09-06

### Added

- Saved European, U.S., and ISO timestamp formats with independent 12-/24-hour clocks, respecting the selected timezone.
- Independent saved cost and name sorting for Models and Sessions, plus newest/oldest session starts; missing costs and start times remain last.
- Precise session timestamps in JSON/CSV exports and scrollable help for compact terminals.
- Native ccusage bundled for all six release platforms, with verified dependency integrity.
- Local model prices and background refresh for faster uncached loads, with price dates and explicit incomplete-cost coverage.
- A checksum-verifying macOS/Linux release updater that keeps the complete bundle together and preserves preferences.

### Changed

- Default reports no longer wait for online model pricing. Custom ccusage configurations and explicitly selected backends retain their original pricing behavior.
- JSON exports include price provenance and models with unavailable costs.

### Maintenance and security

- Upgrade Go dependencies, including `golang.org/x/image` to 0.41.0 to address two reported TIFF decoding vulnerabilities.
- Add contribution, conduct, security, support, and governance policies, issue forms, review ownership, and dependency-update automation.

### Upgrade notes

- Extract the complete release archive and keep `libexec` beside the Tokenlens executable. The macOS/Linux updater installs the whole bundle; replacing only the executable omits the bundled backend. Source installs still need Bun or ccusage.
- Existing configurations inherit European dates, a 24-hour clock, and cost-descending sorting for Models/Sessions. Use `Shift+D`, `Shift+H`, and `s` to change these saved choices.
- CSV exports append `first_activity`, `last_activity`, `snapshot_time`, and `price_date` columns. Consumers expecting a fixed column count should use header names. JSON adds session timestamps and price provenance; timestamp precision is preserved.

## [0.3.0] - 2026-09-06

### Added

- Remember currency, applied theme, daily/weekly/monthly grouping, cost/token
  display, compact numbers, and overview layout across launches.
- Save preferences in `~/.config/tokenlens/config.toml` on macOS and Linux,
  respecting `XDG_CONFIG_HOME`, and in `%AppData%` on Windows.
- `tokenlens config path` and `tokenlens config reset` commands. Explicit CLI
  and environment overrides remain temporary; theme previews do not save.

## [0.2.0] - 2026-09-06

### Added

- GoReleaser builds for macOS, Linux, and Windows on amd64 and arm64, with
  release archives, SHA-256 checksums, and automated GitHub publishing.
- Repeatable semantic release preparation and validation commands.
- Tokyo Night Dark and Solarized Dark palettes.
- Searchable theme popup with fuzzy filtering, keyboard navigation, live preview,
  Enter to apply, and Escape to restore the previous theme. Open with Ctrl+T.

### Changed

- Default to the system timezone instead of UTC for calendar ranges and backend
  reports. `TZ` and `--timezone` remain explicit overrides.
- Shift+T opens the theme picker instead of cycling palettes.
  Use Ctrl+T to search and preview themes; Enter applies and Escape cancels.
- To preserve the old UTC behavior, pass `--timezone UTC`.
- Refreshed the README demo and added release, stars, and contributor badges.

## [0.1.0] - 2026-09-06

### Added

- First public release of Tokenlens.
- Terminal dashboard for agent usage, models, tokens, cache, and sessions.
- Date ranges, independent filters, currency conversion, cached reports, and exports.
- Synthetic demo mode.
- Nine display modes, including Nord, Gruvbox, Tokyo Night Light, Dracula,
  Catppuccin Mocha, and Solarized Light, switchable with Shift+T.
- Prominent active-theme name and cycle position in full and compact dashboard headers.
- `--version` to print the application version without starting the dashboard.

[0.2.0]: https://github.com/Kameleon21/tokenlens/releases/tag/v0.2.0
[0.1.0]: https://github.com/Kameleon21/tokenlens/releases/tag/v0.1.0

[0.3.0]: https://github.com/Kameleon21/tokenlens/releases/tag/v0.3.0

[0.4.0]: https://github.com/Kameleon21/tokenlens/releases/tag/v0.4.0

[0.4.1]: https://github.com/Kameleon21/tokenlens/releases/tag/v0.4.1

[0.5.0]: https://github.com/Kameleon21/tokenlens/releases/tag/v0.5.0
