# Branch: initial-implementation — State

## Goal
Build `ccsessions`: a Go CLI that renders Claude Code JSONL session data into per-session
markdown + json files Claude Code can read with normal tools. One-time bulk render + a
`watch` daemon. Distributed via goreleaser (Releases + Homebrew tap).

Plan: `~/.claude/plans/immutable-swinging-sphinx.md`

## Current status
ALL MILESTONES M0–M4 COMPLETE. Awaiting user review. Nothing committed yet.

Full pipeline works end-to-end:
- `ccsessions render` — bulk/single, incremental, markdown+json, images extracted, subagents linked. Verified on real ~/.claude (46 sessions → 585 files, 198MB).
- `ccsessions watch` — fsnotify daemon, per-session debounce, real-time re-render. Verified live.
- `ccsessions list` / `version`.
- Distribution: goreleaser (4 platforms) + homebrew cask + CI/release workflows. `goreleaser check` clean, snapshot build works.

Packages: config, cli, model, rawjsonl, parser, discover, render, pipeline, watch.
Tests: unit (config/cli/parser/render/pipeline/watch) + golden (render) + live smoke (parser, -tags live). `go test -race ./...` green.

## To do before merge
- Commit (branch initial-implementation covers all of M0–M4; consider renaming).
- Optionally: create armandmcqueen/homebrew-tap repo + HOMEBREW_TAP_TOKEN secret before first release tag.
- 2/154 Agent calls didn't prompt-match a subagent (edge cases) — acceptable; unlinked subagents still render as standalone files.

## Key decisions
- Module: `github.com/armandmcqueen/ccsessions`, go 1.25. Root `main.go` → `internal/cli.Execute()`.
- CLI: cobra. Watcher: fsnotify. Config: hand-rolled (flag>env>default), no Viper.
- Output default `~/.ai/claude-sessions/`, mirrors `<project_key>/<session_id>.{md,json}`.
- Full fidelity. Subagents → separate linked files. Images → extracted real files.
- Compaction via explicit `compact_boundary` system entries (not token-drop heuristic).
- Subagent linkage is PATH-based (`<sid>/subagents/agent-<agentId>.jsonl`); in-file sessionId is the parent's (foreign, don't trust).

## Layout (target)
internal/{model,rawjsonl,parser,discover,render,pipeline,watch,config,cli}

## Known issues / open
- Exact Agent-tool-call ↔ subagent linkage (sourceToolAssistantUUID) to verify on real data in M1.
