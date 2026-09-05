# Loading performance

## Changes implemented

Tokenlens previously started ccusage on every startup, range selection, refresh, and currency switch. Disk caching hid part of the wait but never avoided the backend work.

Reports now have a configurable reuse window (five minutes by default). A fresh exact-range report can complete a load without launching ccusage. Up to 16 visited ranges are retained in memory; disk reports also survive restarts. Old reports remain visible during revalidation, and `r` bypasses freshness. Repeated refreshes of the range already loading do not cancel and restart it. Exchange-rate requests have their own cancellation and response IDs, so changing currency does not reload usage.

This trades immediate freshness for responsiveness within the configured window. The cached label and original timestamp remain visible. Use `r` or `--cache-ttl 0` when you need an updated report. No-cache mode disables both report caches.

These changes do not make an uncached ccusage invocation faster. A first-ever Bun download, log scan, and online pricing lookup can still take time. Installed binaries avoid Go compilation on each launch. The explicit `--offline` option can avoid online pricing work, with different estimates possible.

## Next step: a local usage engine (proposal)

1. Introduce a provider boundary returning normalized usage records, keeping ccusage as a compatibility option.
2. Add a native reader for one agent first. Validate against synthetic fixtures and ccusage totals before offering it as the default. Preserve unknown values, duplicate-event handling, cache-token categories, and session attribution.
3. Build a local incremental index, such as SQLite. Track file identity and offsets; handle appended records, incomplete final lines, truncation, rotation, edits, and deleted files. The first scan builds the index; later refreshes process changes.
4. Query dates, models, agents, and sessions locally. Keep timestamped events so timezone boundaries and sessions spanning multiple dates remain correct. Daily totals alone cannot reconstruct these views safely.
5. Separate pricing from usage ingestion. Cache versioned model prices, show their source and age, and keep unknown costs unavailable. Refresh prices independently from token counts.

This would make Tokenlens own ingestion, storage, and querying as well as visualization. It also creates maintenance responsibility for agent log formats and pricing semantics. Implement and validate one provider at a time.

## Measuring improvements

Measure first-ever startup, startup with installed backend, warm restart, return to a visited range, unseen range, forced refresh, and currency switch separately. Record time to first visible report and time to updated data; these are different outcomes. Use synthetic fixtures for automated tests and do not commit private reports or machine-specific timing promises.

`go test -run TestRecentReportSkipsBackend -v` verifies disk and memory reuse with a nonexistent backend, so a pass proves those paths do not need ccusage. The full suite also checks freshness boundaries, bounded memory, stale-response rejection, refresh deduplication, and currency isolation.
