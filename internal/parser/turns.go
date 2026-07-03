package parser

import (
	"strings"

	"github.com/armandmcqueen/ccsessions/internal/model"
	"github.com/armandmcqueen/ccsessions/internal/rawjsonl"
)

// buildTurns walks the entry stream in order and assembles turns. A user message
// that carries text starts a new turn; assistant messages become responses in the
// current turn; tool_result-only user messages are not turn boundaries (their
// content was already indexed by collectToolResults). Assistant/system entries
// that appear before the first user message are attached to a synthetic turn 0
// rather than dropped.
func buildTurns(s *model.Session, entries []*rawjsonl.Entry, results map[string]toolResult) {
	var turns []model.Turn
	var pendingCompaction *model.Compaction

	// ensureTurn returns a pointer to the current open turn, creating a synthetic
	// preamble turn if assistant/system content arrives before any user text.
	current := func() *model.Turn {
		if len(turns) == 0 {
			turns = append(turns, model.Turn{Index: 0})
			if pendingCompaction != nil {
				turns[0].CompactionBefore = pendingCompaction
				pendingCompaction = nil
			}
		}
		return &turns[len(turns)-1]
	}

	for _, e := range entries {
		switch e.Type {
		case "user":
			text, isTurn := userTurnText(e)
			if !isTurn {
				continue // tool_result-only message
			}
			turns = append(turns, model.Turn{
				Index:     len(turns),
				UserText:  text,
				Timestamp: parseTime(e.Timestamp),
			})
			if pendingCompaction != nil {
				turns[len(turns)-1].CompactionBefore = pendingCompaction
				pendingCompaction = nil
			}

		case "assistant":
			resp := buildResponse(e, results)
			t := current()
			t.Responses = append(t.Responses, resp)

		case "system":
			switch e.Subtype {
			case "turn_duration":
				if len(turns) > 0 {
					turns[len(turns)-1].DurationMs = e.DurationMs
				}
			case "compact_boundary":
				c := &model.Compaction{Timestamp: parseTime(e.Timestamp)}
				if e.CompactMetadata != nil {
					c.Trigger = e.CompactMetadata.Trigger
					c.PreTokens = e.CompactMetadata.PreTokens
				}
				pendingCompaction = c
			}
		}
	}

	s.Turns = turns
}

// userTurnText extracts the joined text of a user message and reports whether the
// message should start a turn (i.e. it carries actual text, not only tool results).
func userTurnText(e *rawjsonl.Entry) (string, bool) {
	msg, err := e.ParseMessage()
	if err != nil || msg == nil {
		return "", false
	}
	blocks, err := msg.Blocks()
	if err != nil {
		return "", false
	}
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
			parts = append(parts, b.Text)
		}
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, "\n"), true
}

// buildResponse converts an assistant entry into an AssistantResponse, pairing
// each tool_use with its result from the pre-collected results map.
func buildResponse(e *rawjsonl.Entry, results map[string]toolResult) model.AssistantResponse {
	resp := model.AssistantResponse{
		RequestID: e.RequestID,
		Timestamp: parseTime(e.Timestamp),
	}
	msg, err := e.ParseMessage()
	if err != nil || msg == nil {
		return resp
	}
	resp.Model = msg.Model
	resp.StopReason = msg.StopReason
	if msg.Usage != nil {
		resp.Usage = model.Usage{
			InputTokens:         msg.Usage.InputTokens,
			OutputTokens:        msg.Usage.OutputTokens,
			CacheReadTokens:     msg.Usage.CacheReadTokens,
			CacheCreationTokens: msg.Usage.CacheCreationTokens,
			ServiceTier:         msg.Usage.ServiceTier,
		}
	}
	blocks, err := msg.Blocks()
	if err != nil {
		return resp
	}
	for _, b := range blocks {
		switch b.Type {
		case "text":
			resp.TextBlocks = append(resp.TextBlocks, model.TextBlock{Text: b.Text})
		case "thinking":
			resp.ThinkingBlocks = append(resp.ThinkingBlocks, model.ThinkingBlock{Signature: b.Signature})
		case "tool_use":
			tc := model.ToolCall{
				Name:      b.Name,
				ToolUseID: b.ID,
				Input:     b.Input,
			}
			if r, ok := results[b.ID]; ok {
				tc.Result = r.text
				tc.ResultImages = r.images
				tc.IsError = r.isError
			}
			resp.ToolCalls = append(resp.ToolCalls, tc)
		}
	}
	return resp
}
