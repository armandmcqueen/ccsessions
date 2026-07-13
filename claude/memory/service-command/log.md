# Branch: service-command — Log

## 2026-07-13 — service command
- Branched from main (after correcting a stale-main mishap: fetched origin so main=3978ed2 merged PR #1, re-branched).
- Added github.com/kardianos/service v1.3.0.
- internal/cli/service.go: service command group (install/uninstall/start/stop/status/run). watchProgram implements service.Interface, reuses internal/watch.Watcher. serviceConfig bakes resolved config into launch args; per-user launchd/systemd; logs to ~/Library/Logs/ccsessions.log or cache dir.
- Wired into root; README + DESIGN updated; service_test.go added.
- Verified: go test -race green; installed live → replicating 126 sessions to ~/data/ccsessions (582 md + 582 json, 441MB), real-time renders confirmed in log.
