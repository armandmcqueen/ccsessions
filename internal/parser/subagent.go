package parser

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/armandmcqueen/ccsessions/internal/model"
)

// AgentIDFromPath extracts the subagent id from a path like
// ".../subagents/agent-<id>.jsonl".
func AgentIDFromPath(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	return strings.TrimPrefix(base, "agent-")
}

// ParseSubagentFile parses a subagent transcript. The session id inside the file
// is the parent's (foreign), so identity comes from the filename instead.
func ParseSubagentFile(path, projectKey string) (*model.Session, error) {
	s, err := ParseFile(path, projectKey)
	if err != nil {
		return nil, err
	}
	s.IsSubagent = true
	s.AgentID = AgentIDFromPath(path)
	s.SessionID = s.AgentID // do not trust the foreign in-file sessionId
	return s, nil
}

// LinkSubagents associates each subagent with the parent Agent/Task tool call
// that spawned it, by matching the subagent's first user message (the task
// prompt) against the tool call's input.prompt. On a match it records the
// subagent id on the tool call so renderers can emit a link. Subagents that do
// not match any call are left unlinked (renderers list them separately).
func LinkSubagents(parent *model.Session, subs []*model.Session) {
	// Index subagents by a normalized form of their first user text.
	byPrompt := make(map[string]*model.Session, len(subs))
	for _, sub := range subs {
		key := normalizePrompt(sub.FirstUserText())
		if key != "" {
			byPrompt[key] = sub
		}
	}
	if len(byPrompt) == 0 {
		return
	}
	for ti := range parent.Turns {
		for ri := range parent.Turns[ti].Responses {
			calls := parent.Turns[ti].Responses[ri].ToolCalls
			for ci := range calls {
				tc := &calls[ci]
				if tc.Name != "Agent" && tc.Name != "Task" {
					continue
				}
				prompt := normalizePrompt(toolInputString(tc.Input, "prompt"))
				if prompt == "" {
					continue
				}
				if sub, ok := byPrompt[prompt]; ok {
					tc.SubagentID = sub.AgentID
				}
			}
		}
	}
}

// toolInputString pulls a string field out of a tool call's raw input json.
func toolInputString(raw json.RawMessage, field string) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	v, ok := m[field]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		return ""
	}
	return s
}

// normalizePrompt collapses whitespace so minor formatting differences between
// the stored Agent prompt and the subagent's echoed first message still match.
func normalizePrompt(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
