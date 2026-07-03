# ccsessions

Convert Claude Code session data into per-session files that are easy to read with
normal file tools (`Read`, `grep`, an editor) instead of raw JSONL.

Claude Code stores each session as append-only JSONL under
`~/.claude/projects/<project>/<session>.jsonl`. Those files interleave many record
types and deeply nested content, which makes them awkward to read directly.
`ccsessions` renders each session into clean **markdown** and **json** files.

## Install

### Homebrew (macOS / Linux)

```sh
brew install armandmcqueen/tap/ccsessions
```

### Prebuilt binaries

Download a tarball for your platform from the
[Releases](https://github.com/armandmcqueen/ccsessions/releases) page
(darwin/linux × amd64/arm64), extract, and put `ccsessions` on your `PATH`.

### From source

```sh
go install github.com/armandmcqueen/ccsessions@latest
```

## Usage

```sh
# One-time render of every session into ~/.ai/claude-sessions/
ccsessions render

# Render a single session
ccsessions render <session-id>

# Keep rendered files current in real time
ccsessions watch

# List discovered sessions
ccsessions list
```

### Configuration

| Setting     | Flag           | Env                     | Default                  |
| ----------- | -------------- | ----------------------- | ------------------------ |
| Claude home | `--claude-dir` | `CCSESSIONS_CLAUDE_DIR` | `~/.claude`              |
| Output dir  | `--out`        | `CCSESSIONS_OUT`        | `~/.ai/claude-sessions`  |
| Formats     | `--format`     | `CCSESSIONS_FORMAT`     | `markdown,json`          |

Precedence is flag > environment variable > default.

## Development

```sh
go build ./...
go vet ./...
go test ./...
```

### Output layout

```
~/.ai/claude-sessions/
  <project_key>/
    <session_id>.md                       # main transcript (markdown)
    <session_id>.json                     # main transcript (parsed model as json)
    <session_id>.agent-<agentId>.md       # each subagent, linked from the parent
    <session_id>.agent-<agentId>.json
    <session_id>.assets/img-<hash>.png    # images extracted from tool results
```

Rendering is incremental: a session is re-rendered only when its source JSONL
(or a subagent's) is newer than the existing output. Use `--force` to override.

### Releasing

Releases are cut by goreleaser on a `vX.Y.Z` tag (`.github/workflows/release.yml`).
Publishing the Homebrew cask requires a separate `armandmcqueen/homebrew-tap`
repository and a `HOMEBREW_TAP_TOKEN` secret (a PAT with `contents:write` on the
tap repo — the default `GITHUB_TOKEN` cannot push to another repository).

Validate and dry-run locally:

```sh
goreleaser check
goreleaser release --snapshot --clean
```

See [DESIGN.md](DESIGN.md) for architecture.
