//go:build live

// Live smoke test: parses every session in a real Claude home and asserts the
// parser never panics or errors. Opt in with:
//
//	CCSESSIONS_TEST_CLAUDE_DIR=~/.claude go test -tags live ./internal/parser/
package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/armandmcqueen/ccsessions/internal/discover"
	"github.com/armandmcqueen/ccsessions/internal/model"
)

func TestLiveParseAll(t *testing.T) {
	dir := os.Getenv("CCSESSIONS_TEST_CLAUDE_DIR")
	if dir == "" {
		t.Skip("set CCSESSIONS_TEST_CLAUDE_DIR to run the live smoke test")
	}
	if len(dir) >= 2 && dir[:2] == "~/" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, dir[2:])
	}

	refs, err := discover.Sessions(dir, nil)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(refs) == 0 {
		t.Fatalf("no sessions found under %s", dir)
	}
	t.Logf("parsing %d sessions", len(refs))

	var turns, subFiles, agentCalls, linked int
	for _, ref := range refs {
		s, err := ParseFile(ref.MainPath, ref.ProjectKey)
		if err != nil {
			t.Errorf("parse %s: %v", ref.MainPath, err)
			continue
		}
		turns += len(s.Turns)

		var subs []*model.Session
		for _, sp := range ref.SubagentFiles() {
			ss, err := ParseSubagentFile(sp, ref.ProjectKey)
			if err != nil {
				t.Errorf("parse subagent %s: %v", sp, err)
				continue
			}
			subFiles++
			subs = append(subs, ss)
		}
		LinkSubagents(s, subs)

		for _, tn := range s.Turns {
			for _, r := range tn.Responses {
				for _, c := range r.ToolCalls {
					if c.Name == "Agent" || c.Name == "Task" {
						agentCalls++
					}
					if c.SubagentID != "" {
						linked++
					}
				}
			}
		}
	}
	t.Logf("ok: %d turns, %d subagent files, %d Agent/Task calls, %d linked to subagents",
		turns, subFiles, agentCalls, linked)
}
