# Branch: service-command — State

## Goal
Add a `service` command group so ccsessions can run `watch` as an OS-managed
background service (continuous replication without a terminal). Requested to
replicate sessions constantly into `~/data/ccsessions`.

## Status
COMPLETE and installed live. Ready to commit + PR.
- `ccsessions service install|uninstall|start|stop|status|run` (run is hidden).
- Uses github.com/kardianos/service v1.3.0 (launchd/systemd/windows; pure Go).
- Per-user service (Option UserService+RunAtLoad+KeepAlive) — no root, auto-start at login, restart on crash.
- `install` bakes resolved config (--out/--claude-dir/--debounce/--format/--project) into service Arguments.
- Control ops (start/stop/status/uninstall) identify by Name only (newControlService), don't take output flags.
- Service logs → ~/Library/Logs/ccsessions.log (macOS) / user cache dir (Linux).

## Verified live
- Installed pointing at ~/data/ccsessions. `service status` → running.
- Replicated 126 sessions → 582 md + 582 json (441MB). Log shows real-time renders as sessions change (incl. the active session each turn).
- Full `go test -race ./...` green (added service_test.go: arg-baking + log path).

## Notes
- Session count jumped 46 → 126: the earlier cleanupPeriodDays=3650 fix stopped Claude Code's 30-day pruning; sessions now accumulate.
- Global binary rebuilt at ~/.local/bin/ccsessions-v0.0.1 (symlink ccsessions→it) with the service command.
- Base fix: this branch was briefly created from a stale local main (pre-fetch, empty tree); corrected by fetching origin (main→3978ed2, the merged PR #1) and re-branching.

## Key decisions
- kardianos/service over hand-rolled launchd (cross-platform, less code).
- Per-user (not system) service — no sudo.
- `service run` reuses internal/watch.Watcher; no logic duplication.
