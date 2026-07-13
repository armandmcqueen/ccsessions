// Package repogroup maps a session's working directory to a stable grouping key
// so sessions from many project directories that refer to the same git repo
// (e.g. multiple worktrees) are collected together.
package repogroup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Mode selects how output is grouped.
const (
	ModeRepo    = "repo"    // group by resolved git repo (default)
	ModeProject = "project" // keep the original path-encoded project_key
)

// Reason codes explain why a session landed in its group.
const (
	ReasonRule             = "rule"              // matched a user-supplied regex rule
	ReasonProjectMode      = "project-mode"      // grouping disabled
	ReasonNoCWD            = "no-cwd"            // no working directory recorded
	ReasonGitRemote        = "git-remote"        // resolved from origin remote
	ReasonGitToplevel      = "git-toplevel"      // resolved from repo root basename (no remote)
	ReasonBasenameIndex    = "basename-index"    // dir gone; merged with a live sibling by basename
	ReasonBasenameFallback = "basename-fallback" // not a git repo / dir gone; grouped by bare basename
)

// resolution is the cached outcome of resolving one working directory.
type resolution struct {
	key    string
	reason string
	detail string
	ok     bool
}

// Grouper resolves working directories to grouping keys. It is safe for
// concurrent use.
type Grouper struct {
	mode    string
	rules   *RuleSet
	resolve func(cwd string) resolution // injectable for tests

	mu     sync.Mutex
	byCwd  map[string]resolution
	byBase map[string]string // repo basename -> repo key (from live resolutions)
}

// New returns a Grouper for the given mode using git-backed resolution.
func New(mode string) *Grouper {
	return &Grouper{
		mode:    mode,
		resolve: gitResolve,
		byCwd:   make(map[string]resolution),
		byBase:  make(map[string]string),
	}
}

// SetRules attaches an ordered rule set that overrides resolution: a session
// whose subject matches a rule is grouped by that rule before any git logic.
func (g *Grouper) SetRules(rs *RuleSet) { g.rules = rs }

// Prime resolves cwd via git (if it still exists) and records the result plus its
// basename index entry. Calling Prime for every session before keying any of them
// lets deleted-directory sessions merge with live siblings by basename.
func (g *Grouper) Prime(cwd string) {
	if g.mode != ModeRepo || cwd == "" {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.resolveLocked(cwd)
}

// Key returns the grouping key for a session's cwd.
func (g *Grouper) Key(cwd, projectKey string) string {
	key, _, _ := g.Explain(cwd, projectKey)
	return key
}

// Explain returns the grouping key together with the reason code and a
// human-readable detail describing why the session was grouped that way.
func (g *Grouper) Explain(cwd, projectKey string) (key, reason, detail string) {
	// User rules take precedence over any built-in resolution. The subject is the
	// cwd when known, otherwise the project key (which path-encodes the cwd).
	if !g.rules.Empty() {
		subject := cwd
		label := "cwd"
		if subject == "" {
			subject, label = projectKey, "project key"
		}
		if grp, pat, ok := g.rules.Apply(subject); ok {
			return grp, ReasonRule, fmt.Sprintf("matched rule /%s/ on %s %q", pat, label, subject)
		}
	}

	if g.mode != ModeRepo {
		return projectKey, ReasonProjectMode, "grouping by project directory"
	}
	if cwd == "" {
		return projectKey, ReasonNoCWD, "no working directory recorded in the session"
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	res := g.resolveLocked(cwd)
	if res.ok {
		return res.key, res.reason, res.detail
	}

	// Directory could not be resolved (gone or not a git repo): recover by basename.
	base := sanitizeSegment(filepath.Base(cwd))
	if base == "" {
		return projectKey, ReasonNoCWD, res.detail
	}
	if key, ok := g.byBase[base]; ok {
		return key, ReasonBasenameIndex, res.detail + "; matched live sibling by basename '" + base + "'"
	}
	return base, ReasonBasenameFallback, res.detail + "; grouped by bare basename '" + base + "'"
}

// resolveLocked resolves and caches cwd; caller must hold g.mu.
func (g *Grouper) resolveLocked(cwd string) resolution {
	if res, ok := g.byCwd[cwd]; ok {
		return res
	}
	res := g.resolve(cwd)
	g.byCwd[cwd] = res
	if res.ok {
		if base := lastSegment(res.key); base != "" {
			if _, exists := g.byBase[base]; !exists {
				g.byBase[base] = res.key
			}
		}
	}
	return res
}

// gitResolve resolves a directory to a repo key: the normalized origin remote if
// present, otherwise the repository root's basename.
func gitResolve(cwd string) resolution {
	if info, err := os.Stat(cwd); err != nil || !info.IsDir() {
		return resolution{ok: false, detail: "directory does not exist on this machine"}
	}
	if remote := runGit(cwd, "config", "--get", "remote.origin.url"); remote != "" {
		if key := normalizeRemote(remote); key != "" {
			return resolution{key: key, reason: ReasonGitRemote, detail: remote, ok: true}
		}
	}
	if top := runGit(cwd, "rev-parse", "--show-toplevel"); top != "" {
		if base := sanitizeSegment(filepath.Base(top)); base != "" {
			return resolution{key: base, reason: ReasonGitToplevel, detail: "git repo root " + top + " (no remote)", ok: true}
		}
	}
	return resolution{ok: false, detail: "not a git repository"}
}

func runGit(cwd string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// normalizeRemote turns a git remote URL into a stable host/owner/name key so
// ssh and https forms of the same repo collapse together. Returns "" if the URL
// cannot be parsed.
//
//	git@github.com:owner/name.git        -> github.com/owner/name
//	https://github.com/owner/name.git    -> github.com/owner/name
//	ssh://git@github.com/owner/name.git  -> github.com/owner/name
func normalizeRemote(url string) string {
	s := strings.TrimSpace(url)
	s = strings.TrimSuffix(s, ".git")

	switch {
	case strings.HasPrefix(s, "ssh://"):
		s = strings.TrimPrefix(s, "ssh://")
	case strings.HasPrefix(s, "https://"):
		s = strings.TrimPrefix(s, "https://")
	case strings.HasPrefix(s, "http://"):
		s = strings.TrimPrefix(s, "http://")
	case strings.HasPrefix(s, "git://"):
		s = strings.TrimPrefix(s, "git://")
	}
	// Strip any userinfo (e.g. "git@").
	if at := strings.Index(s, "@"); at >= 0 && at < strings.IndexAny(s+"/", "/") {
		s = s[at+1:]
	}
	// scp-like "host:owner/name" -> "host/owner/name".
	if !strings.Contains(s, "/") {
		s = strings.Replace(s, ":", "/", 1)
	} else if i := strings.Index(s, ":"); i >= 0 && i < strings.Index(s, "/") {
		s = s[:i] + "/" + s[i+1:]
	}

	var segs []string
	for _, seg := range strings.Split(s, "/") {
		if seg = sanitizeSegment(seg); seg != "" {
			segs = append(segs, seg)
		}
	}
	if len(segs) < 2 {
		return ""
	}
	return strings.Join(segs, "/")
}

// lastSegment returns the final path segment of a "/"-joined key.
func lastSegment(key string) string {
	parts := strings.Split(key, "/")
	return parts[len(parts)-1]
}

// sanitizeSegment makes a string safe to use as a single path segment, guarding
// against path traversal and separators.
func sanitizeSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "." || s == ".." {
		return ""
	}
	s = strings.ReplaceAll(s, string(filepath.Separator), "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	return s
}
