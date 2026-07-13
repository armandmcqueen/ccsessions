# Branch: repo-grouping — Log

## 2026-07-13 — repo grouping
- Branched from service-command (a1b9391).
- New pkg internal/repogroup: Grouper with git-backed resolve (origin remote → host/owner/name; ssh/https collapse), basename fallback, two-pass basename index for deleted worktrees, injectable resolver for tests.
- model.Session +CWD +Repo; parser extracts cwd; discover.PeekCWD for cheap pre-pass.
- pipeline: Options.GroupBy/Grouper; RenderAll primes grouper with all cwds; RenderOne + needsRender group by resolved key; sets session.Repo.
- config: GroupBy (+env CCSESSIONS_GROUP_BY, default "repo"); cfgFromCmd validates repo|project. --group-by flag added to render/watch/service (addOutputFlags); baked into service args.
- README + DESIGN updated. repogroup + config tests added.
- Verified: real render 127 sessions → 17 repo groups; 102 folded into github.com/armandmcqueen/armand.dev. go test -race green.
