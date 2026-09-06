# Changelog

User-visible changes are recorded here. Versions follow the conventions in
[the release guide](docs/releases.md).

## Unreleased

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
