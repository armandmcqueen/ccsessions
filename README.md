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

### Run it constantly (background service)

To keep sessions replicated to a location without leaving a terminal open, install
ccsessions as an OS-managed background service (launchd on macOS, systemd on Linux).
It auto-starts at login and restarts on crash.

```sh
# Install + start, replicating into ~/data/ccsessions
ccsessions service install --out ~/data/ccsessions

ccsessions service status        # running / stopped / not installed
ccsessions service stop          # pause without removing
ccsessions service start         # resume
ccsessions service uninstall     # remove entirely
```

`install` bakes the current `--out`, `--claude-dir`, `--format`, `--debounce`, and
`--project` settings into the service definition. Service logs are written to
`~/Library/Logs/ccsessions.log` (macOS) or the user cache dir (Linux).

### Configuration

| Setting     | Flag           | Env                     | Default                  |
| ----------- | -------------- | ----------------------- | ------------------------ |
| Claude home | `--claude-dir` | `CCSESSIONS_CLAUDE_DIR` | `~/.claude`              |
| Output dir  | `--out`        | `CCSESSIONS_OUT`        | `~/.ai/claude-sessions`  |
| Formats     | `--format`     | `CCSESSIONS_FORMAT`     | `markdown,json`          |
| Grouping    | `--group-by`   | `CCSESSIONS_GROUP_BY`   | `repo`                   |

Precedence is flag > environment variable > default.

### Grouping by repo

Claude Code creates a separate project directory for every working directory, so
multiple worktrees or checkouts of the same repo end up as many unrelated project
folders. By default ccsessions folds them back together: each session's working
directory is resolved to its git repo and output is grouped as
`<host>/<owner>/<name>/<session_id>.*` (e.g. all worktrees of `armand.dev` land in
`github.com/you/armand.dev/`). ssh and https remotes collapse to the same key.

- Directories that aren't git repos fall back to the directory's basename.
- Directories that no longer exist (deleted worktrees) are matched by basename to a
  living sibling of the same repo, so historical sessions still group correctly.
- Use `--group-by project` to keep the original path-encoded project directories.

### Custom grouping rules

Git resolution can't cover every case — cloud worktrees with no recorded working
directory, deleted worktrees, or non-git directories. Instead of guessing, supply
explicit regex rules with `--group-rules <file>` (env `CCSESSIONS_GROUP_RULES`).
Rules are tried in order; the first match wins and **overrides** git resolution.

```json
{
  "rules": [
    { "pattern": "armand[-.]dev-workdirs", "group": "github.com/me/armand.dev-workdirs" },
    { "pattern": "armand[-.]dev",          "group": "github.com/me/armand.dev" },
    { "pattern": "(/|-)browserbase(/|$|-)", "group": "browserbase" }
  ]
}
```

- Each rule matches the session's working directory, or its path-encoded project
  key when no cwd was recorded (so `.` in a pattern conveniently matches both the
  `/` in a real path and the `-` in an encoded key).
- `group` may reference capture groups (`$1`, `${name}`).
- Order matters — put more specific patterns first (e.g. `…dev-workdirs` before
  `…dev`).

Preview any layout before committing to it with the audit command:

```sh
ccsessions audit                                    # current grouping, with reasons
ccsessions audit --group-rules rules.json           # preview a rules file
ccsessions audit --json                             # machine-readable
```

`audit` prints, per group, which directories are folded in, how many sessions
each contributes, and why — flagging groups that fell back to a bare basename or
had no cwd (the fragile/uncertain ones) with ⚠.

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
