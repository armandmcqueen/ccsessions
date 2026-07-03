package parser

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/armandmcqueen/ccsessions/internal/model"
)

// line marshals a map into a single JSONL line.
func line(m map[string]any) string {
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// jsonl joins lines into a JSONL document.
func jsonl(lines ...string) string { return strings.Join(lines, "\n") + "\n" }

func userText(text string) string {
	return line(map[string]any{
		"type":      "user",
		"timestamp": "2026-06-01T00:00:00.000Z",
		"message":   map[string]any{"role": "user", "content": text},
	})
}

func assistant(blocks ...map[string]any) string {
	return line(map[string]any{
		"type":      "assistant",
		"timestamp": "2026-06-01T00:00:01.000Z",
		"requestId": "req_1",
		"message": map[string]any{
			"role":        "assistant",
			"model":       "claude-opus-4-8",
			"stop_reason": "end_turn",
			"usage":       map[string]any{"input_tokens": 10, "output_tokens": 5},
			"content":     blocks,
		},
	})
}

func toolResultLine(id, text string, isErr bool) string {
	return line(map[string]any{
		"type": "user",
		"message": map[string]any{"role": "user", "content": []map[string]any{{
			"type": "tool_result", "tool_use_id": id, "content": text, "is_error": isErr,
		}}},
	})
}

func TestStringUserContent(t *testing.T) {
	sess, _ := ParseReader(strings.NewReader(jsonl(userText("hello world"))), "s", "-p")
	if len(sess.Turns) != 1 || sess.Turns[0].UserText != "hello world" {
		t.Fatalf("turns=%+v", sess.Turns)
	}
}

func TestToolUseMatchesResult(t *testing.T) {
	doc := jsonl(
		userText("run it"),
		assistant(map[string]any{"type": "tool_use", "id": "tu_1", "name": "Bash", "input": map[string]any{"command": "ls"}}),
		toolResultLine("tu_1", "file.txt", false),
	)
	sess, _ := ParseReader(strings.NewReader(doc), "s", "-p")
	if len(sess.Turns) != 1 {
		t.Fatalf("want 1 turn, got %d", len(sess.Turns))
	}
	calls := sess.Turns[0].Responses[0].ToolCalls
	if len(calls) != 1 || calls[0].Result != "file.txt" || calls[0].Name != "Bash" {
		t.Fatalf("tool call = %+v", calls)
	}
}

func TestErrorResult(t *testing.T) {
	doc := jsonl(
		userText("x"),
		assistant(map[string]any{"type": "tool_use", "id": "t", "name": "Bash", "input": map[string]any{}}),
		toolResultLine("t", "boom", true),
	)
	sess, _ := ParseReader(strings.NewReader(doc), "s", "-p")
	if !sess.Turns[0].Responses[0].ToolCalls[0].IsError {
		t.Fatal("expected IsError")
	}
}

func TestThinkingBlock(t *testing.T) {
	doc := jsonl(
		userText("x"),
		assistant(
			map[string]any{"type": "thinking", "thinking": "", "signature": "sig123"},
			map[string]any{"type": "text", "text": "done"},
		),
	)
	sess, _ := ParseReader(strings.NewReader(doc), "s", "-p")
	r := sess.Turns[0].Responses[0]
	if len(r.ThinkingBlocks) != 1 || r.ThinkingBlocks[0].Signature != "sig123" {
		t.Fatalf("thinking = %+v", r.ThinkingBlocks)
	}
	if len(r.TextBlocks) != 1 || r.TextBlocks[0].Text != "done" {
		t.Fatalf("text = %+v", r.TextBlocks)
	}
}

func TestToolResultOnlyUserIsNotATurn(t *testing.T) {
	doc := jsonl(
		userText("first"),
		assistant(map[string]any{"type": "tool_use", "id": "t", "name": "Bash", "input": map[string]any{}}),
		toolResultLine("t", "ok", false),
		assistant(map[string]any{"type": "text", "text": "second response"}),
	)
	sess, _ := ParseReader(strings.NewReader(doc), "s", "-p")
	if len(sess.Turns) != 1 {
		t.Fatalf("tool_result user must not start a turn; got %d turns", len(sess.Turns))
	}
	if len(sess.Turns[0].Responses) != 2 {
		t.Fatalf("want 2 responses in turn, got %d", len(sess.Turns[0].Responses))
	}
}

func TestOrphanAssistantBeforeUser(t *testing.T) {
	doc := jsonl(
		assistant(map[string]any{"type": "text", "text": "preamble"}),
		userText("real question"),
	)
	sess, _ := ParseReader(strings.NewReader(doc), "s", "-p")
	if len(sess.Turns) != 2 {
		t.Fatalf("want synthetic turn 0 + real turn, got %d", len(sess.Turns))
	}
	if sess.Turns[0].UserText != "" || len(sess.Turns[0].Responses) != 1 {
		t.Fatalf("synthetic preamble turn wrong: %+v", sess.Turns[0])
	}
}

func TestMetadataAndCompaction(t *testing.T) {
	doc := jsonl(
		line(map[string]any{"type": "ai-title", "aiTitle": "My Session"}),
		line(map[string]any{"type": "pr-link", "prNumber": 7, "prUrl": "http://x/7"}),
		line(map[string]any{"type": "system", "subtype": "compact_boundary",
			"compactMetadata": map[string]any{"trigger": "manual", "preTokens": 1000}}),
		userText("after compaction"),
	)
	sess, _ := ParseReader(strings.NewReader(doc), "s", "-p")
	if sess.Meta.Title != "My Session" || sess.Meta.PRNumber != 7 {
		t.Fatalf("meta = %+v", sess.Meta)
	}
	if sess.Turns[0].CompactionBefore == nil || sess.Turns[0].CompactionBefore.PreTokens != 1000 {
		t.Fatalf("compaction = %+v", sess.Turns[0].CompactionBefore)
	}
}

func TestTurnDuration(t *testing.T) {
	doc := jsonl(
		userText("x"),
		assistant(map[string]any{"type": "text", "text": "y"}),
		line(map[string]any{"type": "system", "subtype": "turn_duration", "durationMs": 4242}),
	)
	sess, _ := ParseReader(strings.NewReader(doc), "s", "-p")
	if sess.Turns[0].DurationMs != 4242 {
		t.Fatalf("duration = %d", sess.Turns[0].DurationMs)
	}
}

func TestSubagentLinkage(t *testing.T) {
	parentDoc := jsonl(
		userText("do research"),
		assistant(map[string]any{"type": "tool_use", "id": "tu", "name": "Agent",
			"input": map[string]any{"prompt": "Investigate the  thing", "subagent_type": "Explore"}}),
		toolResultLine("tu", "agent done", false),
	)
	subDoc := jsonl(userText("Investigate the thing"))

	parent, _ := ParseReader(strings.NewReader(parentDoc), "parent", "-p")
	sub, _ := ParseReader(strings.NewReader(subDoc), "agent-abc", "-p")
	sub.IsSubagent = true
	sub.AgentID = "abc"

	LinkSubagents(parent, []*model.Session{sub})
	got := parent.Turns[0].Responses[0].ToolCalls[0].SubagentID
	if got != "abc" {
		t.Fatalf("SubagentID = %q, want abc (whitespace-normalized prompt match)", got)
	}
}

func TestAgentIDFromPath(t *testing.T) {
	if id := AgentIDFromPath("/x/subagents/agent-deadbeef.jsonl"); id != "deadbeef" {
		t.Fatalf("AgentIDFromPath = %q", id)
	}
}
