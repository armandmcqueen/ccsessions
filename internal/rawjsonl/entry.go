// Package rawjsonl models the on-the-wire shape of a single Claude Code session
// JSONL line and handles the string-or-array polymorphism in message content.
package rawjsonl

import "encoding/json"

// Entry is one decoded JSONL line. Only the fields ccsessions needs are modelled;
// unknown fields are ignored. Message is kept raw and decoded on demand because
// its shape depends on the entry type.
type Entry struct {
	Type        string          `json:"type"`
	Subtype     string          `json:"subtype"`
	UUID        string          `json:"uuid"`
	ParentUUID  string          `json:"parentUuid"`
	Timestamp   string          `json:"timestamp"`
	SessionID   string          `json:"sessionId"`
	Version     string          `json:"version"`
	GitBranch   string          `json:"gitBranch"`
	Slug        string          `json:"slug"`
	IsSidechain bool            `json:"isSidechain"`
	RequestID   string          `json:"requestId"`
	CWD         string          `json:"cwd"`
	Message     json.RawMessage `json:"message"`

	// system subtype=turn_duration
	DurationMs int64 `json:"durationMs"`
	// system subtype=compact_boundary
	CompactMetadata *CompactMetadata `json:"compactMetadata"`

	// Metadata record types.
	AiTitle        string `json:"aiTitle"`        // type=ai-title
	AgentName      string `json:"agentName"`      // type=agent-name
	Mode           string `json:"mode"`           // type=mode
	PermissionMode string `json:"permissionMode"` // type=permission-mode
	PRNumber       int    `json:"prNumber"`       // type=pr-link
	PRUrl          string `json:"prUrl"`          // type=pr-link

	// type=fork-context-ref
	ParentSessionID string `json:"parentSessionId"`
}

// CompactMetadata describes a compaction boundary.
type CompactMetadata struct {
	Trigger   string `json:"trigger"`
	PreTokens int    `json:"preTokens"`
}

// Message is the assistant/user message envelope.
type Message struct {
	Role       string          `json:"role"`
	Model      string          `json:"model"`
	ID         string          `json:"id"`
	StopReason string          `json:"stop_reason"`
	Usage      *Usage          `json:"usage"`
	Content    json.RawMessage `json:"content"` // string OR array of ContentBlock
}

// Usage mirrors the message.usage object.
type Usage struct {
	InputTokens         int    `json:"input_tokens"`
	OutputTokens        int    `json:"output_tokens"`
	CacheReadTokens     int    `json:"cache_read_input_tokens"`
	CacheCreationTokens int    `json:"cache_creation_input_tokens"`
	ServiceTier         string `json:"service_tier"`
}

// ContentBlock is a single content block; the populated fields depend on Type
// (text | thinking | tool_use | tool_result | image).
type ContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	Signature string          `json:"signature"`
	ID        string          `json:"id"`    // tool_use
	Name      string          `json:"name"`  // tool_use
	Input     json.RawMessage `json:"input"` // tool_use
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"` // tool_result: string OR array
	IsError   bool            `json:"is_error"`
	Source    *ImageSource    `json:"source"` // image
}

// ImageSource is the source of an image block (base64-encoded data).
type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}
