package model

import (
	"encoding/json"
	"time"
)

// TextBlock is a text content block emitted by the assistant.
type TextBlock struct {
	Text string `json:"text"`
}

// ThinkingBlock is an extended-thinking block. Claude Code redacts the thinking
// text in session logs, preserving only the cryptographic signature, so there is
// no readable content to render — only a marker that thinking occurred.
type ThinkingBlock struct {
	Signature string `json:"signature,omitempty"`
}

// Image is an image that appeared in a tool result or user message. The bytes are
// decoded from base64 at parse time and extracted to a file by the pipeline; Ref
// is filled in with the relative asset path once written.
type Image struct {
	MediaType string `json:"media_type"`
	Data      []byte `json:"-"`             // raw bytes, never marshalled into json output
	Ref       string `json:"ref,omitempty"` // relative path to the extracted asset file
}

// ToolCall is a single tool invocation by the assistant, paired with its result.
type ToolCall struct {
	Name         string          `json:"name"`
	ToolUseID    string          `json:"tool_use_id"`
	Input        json.RawMessage `json:"input"`  // raw json for byte-fidelity
	Result       string          `json:"result"` // flattened text of the tool result
	ResultImages []Image         `json:"result_images,omitempty"`
	IsError      bool            `json:"is_error,omitempty"`
	// SubagentID links an Agent/Task call to the subagent transcript it spawned
	// (the agentId from the subagent filename); empty if no subagent matched.
	SubagentID string `json:"subagent_id,omitempty"`
}

// AssistantResponse is one assistant API response within a turn. A single turn
// can contain several responses when the assistant makes tool calls and is
// re-invoked with the results.
type AssistantResponse struct {
	RequestID      string          `json:"request_id,omitempty"`
	Model          string          `json:"model,omitempty"`
	StopReason     string          `json:"stop_reason,omitempty"`
	Usage          Usage           `json:"usage"`
	TextBlocks     []TextBlock     `json:"text_blocks,omitempty"`
	ThinkingBlocks []ThinkingBlock `json:"thinking_blocks,omitempty"`
	ToolCalls      []ToolCall      `json:"tool_calls,omitempty"`
	Timestamp      time.Time       `json:"timestamp,omitempty"`
}
