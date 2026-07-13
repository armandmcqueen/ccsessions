// Package repogroup maps a session's working directory to a stable grouping key
// so sessions from many project directories that refer to the same git repo
// (e.g. multiple worktrees) are collected together.
package repogroup

import (
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

// Grouper resolves working directories to grouping keys. It is safe for
// concurrent use. Live directories are resolved via git and cached; directories
// that no longer exist fall back to their basename, looked up against an index
// of basenames seen from live repos so deleted worktrees merge with their living
// siblings.
type Grouper struct {
	mode    string
	resolve func(cwd string) (string, bool) // injectable for tests

	mu     sync.Mutex
	byCwd  map[string]string // cwd -> repo key ("" means resolution failed)
	byBase map[string]string // repo basename -> repo key
}

// New returns a Grouper for the given mode using git-backed resolution.
func New(mode string) *Grouper {
	return &Grouper{
		mode:    mode,
		resolve: gitResolve,
		byCwd:   make(map[string]string),
		byBase:  make(map[string]string),
	}
}

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

// Key returns the grouping key for a session's cwd, falling back to projectKey
// when repo resolution is impossible.
func (g *Grouper) Key(cwd, projectKey string) string {
	if g.mode != ModeRepo || cwd == "" {
		return projectKey
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	if key := g.resolveLocked(cwd); key != "" {
		return key
	}
	// Directory is gone: recover via basename, merging with a live sibling if one
	// established this basename, otherwise grouping by the basename alone.
	base := sanitizeSegment(filepath.Base(cwd))
	if base == "" {
		return projectKey
	}
	if key, ok := g.byBase[base]; ok {
		return key
	}
	return base
}

// resolveLocked resolves and caches cwd; caller must hold g.mu. Returns "" if the
// directory cannot be resolved to a repo (e.g. it no longer exists).
func (g *Grouper) resolveLocked(cwd string) string {
	if key, ok := g.byCwd[cwd]; ok {
		return key
	}
	key, ok := g.resolve(cwd)
	if !ok {
		g.byCwd[cwd] = ""
		return ""
	}
	g.byCwd[cwd] = key
	if base := lastSegment(key); base != "" {
		if _, exists := g.byBase[base]; !exists {
			g.byBase[base] = key
		}
	}
	return key
}

// gitResolve resolves a directory to a repo key: the normalized origin remote if
// present, otherwise the repository root's basename. Returns ok=false if the
// directory does not exist or is not a git repository.
func gitResolve(cwd string) (string, bool) {
	if info, err := os.Stat(cwd); err != nil || !info.IsDir() {
		return "", false
	}
	if remote := runGit(cwd, "config", "--get", "remote.origin.url"); remote != "" {
		if key := normalizeRemote(remote); key != "" {
			return key, true
		}
	}
	if top := runGit(cwd, "rev-parse", "--show-toplevel"); top != "" {
		if base := sanitizeSegment(filepath.Base(top)); base != "" {
			return base, true
		}
	}
	return "", false
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

	// Sanitize each path segment and drop empties.
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
