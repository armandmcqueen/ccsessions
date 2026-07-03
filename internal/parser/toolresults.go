package parser

import (
	"github.com/armandmcqueen/ccsessions/internal/model"
	"github.com/armandmcqueen/ccsessions/internal/rawjsonl"
)

// toolResult is a tool_result collected from a user entry, keyed by tool_use_id.
type toolResult struct {
	text    string
	images  []model.Image
	isError bool
}

// collectToolResults scans all user entries for tool_result blocks and indexes
// them by tool_use_id so the turn builder can pair them with tool_use calls. This
// is a separate pass because a result can appear in a later entry than its call.
func collectToolResults(entries []*rawjsonl.Entry) map[string]toolResult {
	results := make(map[string]toolResult)
	for _, e := range entries {
		if e.Type != "user" {
			continue
		}
		msg, err := e.ParseMessage()
		if err != nil || msg == nil {
			continue
		}
		blocks, err := msg.Blocks()
		if err != nil {
			continue
		}
		for _, b := range blocks {
			if b.Type != "tool_result" {
				continue
			}
			text, decoded := rawjsonl.ResultContent(b.Content)
			imgs := make([]model.Image, 0, len(decoded))
			for _, d := range decoded {
				imgs = append(imgs, model.Image{MediaType: d.MediaType, Data: d.Data})
			}
			results[b.ToolUseID] = toolResult{text: text, images: imgs, isError: b.IsError}
		}
	}
	return results
}
