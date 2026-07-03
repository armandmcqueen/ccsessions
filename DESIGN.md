# ccsessions — Design

This document describes the architecture of `ccsessions`. It is the primary artifact
for reviewing the approach and is kept current as the code evolves.

## Problem

Claude Code persists each session as append-only JSONL at
`~/.claude/projects/<project_key>/<session_id>.jsonl`, with subagent transcripts at
`<session_id>/subagents/agent-<agentId>.jsonl`. Each line is one record; types
interleave (`user`, `assistant`, `system`, plus metadata records like `ai-title`,
`mode`, `pr-link`, …). Content is deeply nested and a single session can reach 60+ MB.
This is hard to read with normal file tools. `ccsessions` renders each session into
clean per-session **markdown** and **json**.

## High-level flow

```
discover ──▶ parser ──▶ model.Session ──▶ render (Renderer registry) ──▶ pipeline (atomic writes)
                                                                              ▲
                                                          watch (fsnotify) ───┘  (real-time mode)
```

- **discover** — knows the on-disk layout; enumerates projects, sessions, and the
  source files that make up a session (main jsonl + subagent jsonls); maps source to
  output paths.
- **rawjsonl** — wire-format structs and decoding of a single JSONL line, handling the
  `string`-or-`array` content polymorphism.
- **parser** — turns the raw entry stream into a `model.Session` (turns, assistant
  responses, tool calls matched to results, thinking markers, metadata, compaction
  boundaries).
- **model** — pure, dependency-free data types. The JSON renderer marshals these
  directly, so they carry clean JSON tags.
- **render** — a pluggable `Renderer` interface plus a registry. Renderers are pure
  (`*model.Session -> []Output`); the pipeline owns all filesystem I/O.
- **pipeline** — ties discovery + parse + render together, decides what needs
  re-rendering (mtime-based, stateless), and writes outputs atomically.
- **watch** — an fsnotify daemon that maps changed paths back to sessions and triggers
  debounced re-renders.
- **config** — resolves settings with flag > env > default precedence and expands `~`.
- **cli** — cobra command tree (`render`, `watch`, `list`, `version`).

## Key decisions

- **Single static binary, minimal deps.** cobra for the CLI, fsnotify for watching;
  both pure Go (`CGO_ENABLED=0`). Config resolution is hand-rolled (no Viper).
- **Renderers return `[]Output`, pipeline writes.** Keeps renderers pure/testable and
  centralizes atomic writes (temp file + rename) so Claude Code never reads a partial
  file. `[]Output` (not a single file) is required because image extraction and
  separately-rendered subagents emit additional files.
- **Full fidelity.** Tool results and outputs are never truncated. Images are extracted
  to real files under `<session_id>.assets/` and linked.
- **Subagents are separate linked files.** Associated to a parent **by filesystem path**
  (the `<session_id>/subagents/` directory), never by the in-file `sessionId`, which is
  the parent's id (foreign).
- **Compaction via `compact_boundary`.** Detected from explicit `system` records rather
  than a token-drop heuristic.

## Parsing notes

- A `user` message starts a turn only if it carries text; tool_result-only user
  messages are not turn boundaries (their results are pre-indexed by
  `tool_use_id` and paired back to the assistant's `tool_use` blocks).
- Assistant/system entries before the first user message attach to a synthetic
  turn 0 rather than being dropped.
- `thinking` content is redacted by Claude Code (signature only); rendered as a
  marker, never as fabricated content.
- Compaction comes from explicit `system` `compact_boundary` records.
- Subagents are matched to their spawning `Agent`/`Task` call by comparing the
  call's `input.prompt` to the subagent's first user message (whitespace
  normalized). This is version-independent; the older `progress`-entry linkage no
  longer exists in current Claude Code data.
- Lines are read with `bufio.Reader.ReadBytes` (no 64 KB cap) to handle 60 MB+
  sessions; malformed and truncated-final lines are skipped, not fatal.

## Status / roadmap

All milestones complete:

- **M0:** scaffold, CLI skeleton, config package, CI.
- **M1:** discovery + parser + model (table-driven + live smoke tests).
- **M2:** pluggable renderers (markdown + json), image extraction, subagent
  linking, incremental pipeline with atomic writes.
- **M3:** fsnotify watch daemon with per-session debounce.
- **M4:** goreleaser distribution (GitHub Releases + Homebrew cask).

Detailed parsing notes and data-format findings live in the plan at
`~/.claude/plans/immutable-swinging-sphinx.md`.
