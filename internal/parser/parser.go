// Package parser turns a Claude Code session JSONL stream into a model.Session.
package parser

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/armandmcqueen/ccsessions/internal/model"
	"github.com/armandmcqueen/ccsessions/internal/rawjsonl"
)

// ParseFile reads and parses a session file. sessionID is taken from the
// filename stem and projectKey is supplied by the caller (the directory name).
func ParseFile(path, projectKey string) (*model.Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sessionID := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	s, err := ParseReader(f, sessionID, projectKey)
	if err != nil {
		return nil, err
	}
	s.FilePath = path
	return s, nil
}

// ParseReader parses a session from an io.Reader.
func ParseReader(r io.Reader, sessionID, projectKey string) (*model.Session, error) {
	entries, total, err := rawjsonl.DecodeAll(r)
	if err != nil {
		return nil, err
	}

	s := &model.Session{
		SessionID:        sessionID,
		ProjectKey:       projectKey,
		RawEntryCount:    total,
		ParsedEntryCount: len(entries),
	}

	results := collectToolResults(entries)
	gatherMetadata(s, entries)
	buildTurns(s, entries, results)
	return s, nil
}

// gatherMetadata fills session-level fields from the first useful occurrence of
// version/gitBranch/slug and from the metadata record types.
func gatherMetadata(s *model.Session, entries []*rawjsonl.Entry) {
	for _, e := range entries {
		if s.Version == "" && e.Version != "" {
			s.Version = e.Version
		}
		if s.GitBranch == "" && e.GitBranch != "" {
			s.GitBranch = e.GitBranch
		}
		if s.Slug == "" && e.Slug != "" {
			s.Slug = e.Slug
		}
		if s.CWD == "" && e.CWD != "" {
			s.CWD = e.CWD
		}
		switch e.Type {
		case "ai-title":
			if e.AiTitle != "" {
				s.Meta.Title = e.AiTitle
			}
		case "agent-name":
			if e.AgentName != "" {
				s.Meta.AgentName = e.AgentName
			}
		case "pr-link":
			s.Meta.PRURL = e.PRUrl
			s.Meta.PRNumber = e.PRNumber
		case "mode":
			if e.Mode != "" {
				s.Meta.Mode = e.Mode
			}
		case "permission-mode":
			if e.PermissionMode != "" {
				s.Meta.Permission = e.PermissionMode
			}
		case "fork-context-ref":
			if e.ParentSessionID != "" {
				s.Meta.ForkRef = e.ParentSessionID
			}
		}
	}
}

// parseTime parses an RFC3339(nano) timestamp, returning the zero time on failure.
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
