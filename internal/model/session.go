package model

import "time"

// SessionMeta holds session-level metadata gathered from the non-conversational
// record types (ai-title, agent-name, pr-link, mode, …).
type SessionMeta struct {
	Title      string `json:"title,omitempty"`           // ai-title
	AgentName  string `json:"agent_name,omitempty"`      // agent-name
	PRURL      string `json:"pr_url,omitempty"`          // pr-link
	PRNumber   int    `json:"pr_number,omitempty"`       // pr-link
	Mode       string `json:"mode,omitempty"`            // last mode
	Permission string `json:"permission_mode,omitempty"` // last permission-mode
	ForkRef    string `json:"fork_ref,omitempty"`        // fork-context-ref parent session id
}

// Compaction marks a context-compaction boundary that occurred before a turn.
type Compaction struct {
	Trigger   string    `json:"trigger,omitempty"` // "manual" | "auto"
	PreTokens int       `json:"pre_tokens,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// Turn is one user→assistant exchange. Responses holds every assistant API
// response produced before the next user message.
type Turn struct {
	Index            int                 `json:"index"`
	UserText         string              `json:"user_text"`
	Timestamp        time.Time           `json:"timestamp,omitempty"`
	Responses        []AssistantResponse `json:"responses,omitempty"`
	DurationMs       int64               `json:"duration_ms,omitempty"`
	CompactionBefore *Compaction         `json:"compaction_before,omitempty"`
}

// Session is a fully parsed Claude Code session.
type Session struct {
	SessionID  string      `json:"session_id"`
	ProjectKey string      `json:"project_key"`
	FilePath   string      `json:"file_path"`
	Slug       string      `json:"slug,omitempty"`
	Version    string      `json:"version,omitempty"`
	GitBranch  string      `json:"git_branch,omitempty"`
	Meta       SessionMeta `json:"meta"`
	Turns      []Turn      `json:"turns"`

	// IsSubagent marks a session parsed from a subagents/agent-*.jsonl file,
	// AgentID is that subagent's id (from the filename), and ParentSessionID is
	// the id of the top-level session that owns the subagents directory.
	IsSubagent      bool   `json:"is_subagent,omitempty"`
	AgentID         string `json:"agent_id,omitempty"`
	ParentSessionID string `json:"parent_session_id,omitempty"`

	// Counts for sanity-checking parse coverage.
	RawEntryCount    int `json:"raw_entry_count"`
	ParsedEntryCount int `json:"parsed_entry_count"`
}

// FirstUserText returns the first non-empty user message in the session, used to
// match a subagent transcript back to the Agent tool call that spawned it.
func (s *Session) FirstUserText() string {
	for i := range s.Turns {
		if s.Turns[i].UserText != "" {
			return s.Turns[i].UserText
		}
	}
	return ""
}
