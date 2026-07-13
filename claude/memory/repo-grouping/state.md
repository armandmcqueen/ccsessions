# Branch: repo-grouping — State

## Goal
Normalize output so sessions from many project dirs that refer to the same git
repo (esp. worktrees) group together. Requested because user has many worktrees
of one repo (armand.dev).

## Status
COMPLETE, deployed live as v0.2.0, PR #3 open (https://github.com/armandmcqueen/ccsessions/pull/3, rebased onto main after PR #2 merged).
- Live service redeployed (wipe + restart) with repo grouping → ~/data/ccsessions now repo-grouped: 104 sessions under github.com/armandmcqueen/armand.dev, only 2 cwd-less stub dirs remain, 127 total.
- Bug found + fixed during live test: PeekCWD 20-line cap missed cwd behind many leading metadata/file-history-snapshot lines (session 20fc9717 cwd on line 25) → mis-grouped to project_key. Fixed with byte-level scan up to 10000 lines, huge-line safe (commit d5be079/"Fix PeekCWD...").

## Design (as built)
- Grouping key = normalized git origin remote: host/owner/name (ssh+https collapse). Fallback: repo-root basename → project_key.
- Default `--group-by repo`; `--group-by project` opts out. Env CCSESSIONS_GROUP_BY. Validated in cfgFromCmd.
- Deleted worktree dirs (cwd gone): two-pass — RenderAll primes grouper with all cwds first (builds basename→repoKey index), so dead dirs merge with live siblings by basename; else basename; else project_key.
- Resolution is git-shelled and cached per cwd; lives in pipeline (impure), parser stays pure (just extracts cwd).
- model.Session gains CWD + Repo (Repo stored in json for durability after dir deletion).
- discover.PeekCWD cheaply reads cwd from first ~20 lines (avoids full re-parse in pre-pass).
- New pkg internal/repogroup (Grouper, normalizeRemote, injectable resolver for tests).

## Verified
- go test -race ./... green; repogroup unit tests incl. deleted-dir-merge + ssh/https collapse + traversal safety.
- Real render (127 sessions): 74+ project dirs → 17 repo groups. 102 sessions folded into github.com/armandmcqueen/armand.dev. Non-git dirs → basename (kvstore, browserbase, evernote...).

## Open / next
- Deploy decision: rebuilding + restarting the live service with repo grouping re-renders ~/data/ccsessions into repo/ folders and leaves the old project-key dirs behind (stale) until cleaned. Ask user before deploying + whether to wipe ~/data/ccsessions first.
- Version bump (v0.1.0 → v0.2.0?) + sanitized PR (stacks on PR #2).
