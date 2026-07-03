// Package discover knows the on-disk layout of a Claude home and enumerates the
// projects, sessions, and source files that make up each session.
package discover

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SessionRef identifies a session and the files that comprise it.
type SessionRef struct {
	ProjectKey  string // path-encoded project directory name
	SessionID   string // UUID (filename stem of the main jsonl)
	MainPath    string // absolute path to <session_id>.jsonl
	SubagentDir string // absolute path to <session_id>/subagents (may not exist)
}

// ProjectsDir returns the projects directory inside a Claude home.
func ProjectsDir(claudeDir string) string {
	return filepath.Join(claudeDir, "projects")
}

// Sessions enumerates every session under the Claude home, optionally filtered to
// projects whose key contains any of the given substrings (empty = all). Results
// are sorted by project key then session id for deterministic output.
func Sessions(claudeDir string, projectFilters []string) ([]SessionRef, error) {
	projectsDir := ProjectsDir(claudeDir)
	projDirs, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var refs []SessionRef
	for _, pd := range projDirs {
		if !pd.IsDir() {
			continue
		}
		key := pd.Name()
		if !matchesAny(key, projectFilters) {
			continue
		}
		projPath := filepath.Join(projectsDir, key)
		files, err := os.ReadDir(projPath)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			id := strings.TrimSuffix(f.Name(), ".jsonl")
			refs = append(refs, SessionRef{
				ProjectKey:  key,
				SessionID:   id,
				MainPath:    filepath.Join(projPath, f.Name()),
				SubagentDir: filepath.Join(projPath, id, "subagents"),
			})
		}
	}

	sort.Slice(refs, func(i, j int) bool {
		if refs[i].ProjectKey != refs[j].ProjectKey {
			return refs[i].ProjectKey < refs[j].ProjectKey
		}
		return refs[i].SessionID < refs[j].SessionID
	})
	return refs, nil
}

// RefFor builds a SessionRef for a known project key and session id.
func RefFor(claudeDir, projectKey, sessionID string) SessionRef {
	projPath := filepath.Join(ProjectsDir(claudeDir), projectKey)
	return SessionRef{
		ProjectKey:  projectKey,
		SessionID:   sessionID,
		MainPath:    filepath.Join(projPath, sessionID+".jsonl"),
		SubagentDir: filepath.Join(projPath, sessionID, "subagents"),
	}
}

// RefFromPath maps a changed file path under the Claude home to the session it
// belongs to. It handles both main transcripts
// (projects/<proj>/<sid>.jsonl) and subagent transcripts
// (projects/<proj>/<sid>/subagents/agent-*.jsonl), deriving the session id from
// the path — never from the file's contents. ok is false for unrelated paths.
func RefFromPath(claudeDir, path string) (ref SessionRef, ok bool) {
	if !strings.HasSuffix(path, ".jsonl") {
		return SessionRef{}, false
	}
	rel, err := filepath.Rel(ProjectsDir(claudeDir), path)
	if err != nil {
		return SessionRef{}, false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) < 2 || parts[0] == ".." {
		return SessionRef{}, false
	}
	projectKey := parts[0]
	switch {
	case len(parts) == 2:
		// <proj>/<sid>.jsonl
		sid := strings.TrimSuffix(parts[1], ".jsonl")
		return RefFor(claudeDir, projectKey, sid), true
	case len(parts) == 4 && parts[2] == "subagents":
		// <proj>/<sid>/subagents/agent-*.jsonl
		return RefFor(claudeDir, projectKey, parts[1]), true
	default:
		return SessionRef{}, false
	}
}

// SubagentFiles returns the absolute paths of a session's subagent transcripts,
// sorted by filename for determinism. Returns nil if there is no subagents dir.
func (sr SessionRef) SubagentFiles() []string {
	entries, err := os.ReadDir(sr.SubagentDir)
	if err != nil {
		return nil
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		paths = append(paths, filepath.Join(sr.SubagentDir, e.Name()))
	}
	sort.Strings(paths)
	return paths
}

// SourceFiles returns every file that contributes to the session: the main
// transcript plus all subagent transcripts.
func (sr SessionRef) SourceFiles() []string {
	return append([]string{sr.MainPath}, sr.SubagentFiles()...)
}

func matchesAny(key string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	for _, f := range filters {
		if f == "" || strings.Contains(strings.ToLower(key), strings.ToLower(f)) {
			return true
		}
	}
	return false
}
