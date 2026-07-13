// Package discover knows the on-disk layout of a Claude home and enumerates the
// projects, sessions, and source files that make up each session.
package discover

import (
	"bufio"
	"bytes"
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

// peekCWDMaxLines bounds how far PeekCWD scans. A session can open with many
// metadata records (ai-title, mode, permission-mode, and one file-history-snapshot
// per file touched) before the first entry that carries a cwd, so this needs to be
// generous — the scan is a cheap byte search, not a JSON parse.
const peekCWDMaxLines = 10000

var cwdMarker = []byte(`"cwd":"`)

// PeekCWD cheaply reads a session's working directory by scanning its main
// transcript for a "cwd" field, without fully parsing the file. It uses a
// byte-level search (not JSON decoding) so a single huge line — e.g. a large
// pasted attachment — costs nothing and never overflows a scanner buffer.
// Returns "" if no cwd is found within the scan bound.
func PeekCWD(mainPath string) string {
	f, err := os.Open(mainPath)
	if err != nil {
		return ""
	}
	defer f.Close()
	br := bufio.NewReaderSize(f, 1<<20)
	for i := 0; i < peekCWDMaxLines; i++ {
		line, readErr := br.ReadBytes('\n')
		if cwd := extractCWD(line); cwd != "" {
			return cwd
		}
		if readErr != nil {
			break
		}
	}
	return ""
}

// extractCWD pulls the value of a top-level `"cwd":"..."` field out of a raw JSONL
// line via byte search. Session cwds are filesystem paths without embedded quotes,
// so a simple scan to the closing quote is sufficient.
func extractCWD(line []byte) string {
	idx := bytes.Index(line, cwdMarker)
	if idx < 0 {
		return ""
	}
	rest := line[idx+len(cwdMarker):]
	end := bytes.IndexByte(rest, '"')
	if end <= 0 {
		return ""
	}
	return string(rest[:end])
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
