package render

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/armandmcqueen/ccsessions/internal/model"
)

var update = flag.Bool("update", false, "update golden files")

// sampleSession is a canonical fixture exercising text, thinking, tool calls,
// a subagent link, an error result, an image, compaction, and metadata.
func sampleSession() *model.Session {
	ts := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	return &model.Session{
		SessionID:  "sess-123",
		ProjectKey: "-Users-me-code-proj",
		GitBranch:  "main",
		Version:    "2.1.185",
		Meta: model.SessionMeta{
			Title:    "Sample session",
			PRURL:    "https://github.com/me/proj/pull/9",
			PRNumber: 9,
		},
		Turns: []model.Turn{
			{
				Index:      0,
				UserText:   "Investigate the thing\nwith two lines",
				Timestamp:  ts,
				DurationMs: 1500,
				Responses: []model.AssistantResponse{
					{
						Model:          "claude-opus-4-8",
						StopReason:     "tool_use",
						ThinkingBlocks: []model.ThinkingBlock{{Signature: "sig"}},
						TextBlocks:     []model.TextBlock{{Text: "I'll look into it."}},
						ToolCalls: []model.ToolCall{
							{
								Name:      "Bash",
								ToolUseID: "tu_1",
								Input:     json.RawMessage(`{"command":"ls","description":"list"}`),
								Result:    "file.txt\nother.txt",
							},
							{
								Name:       "Agent",
								ToolUseID:  "tu_2",
								Input:      json.RawMessage(`{"prompt":"Investigate the thing","subagent_type":"Explore"}`),
								Result:     "subagent finished",
								SubagentID: "abc123",
							},
							{
								Name:      "Read",
								ToolUseID: "tu_3",
								Input:     json.RawMessage(`{"file_path":"/nope"}`),
								Result:    "no such file",
								IsError:   true,
								ResultImages: []model.Image{
									{MediaType: "image/png", Data: []byte("PNGDATA")},
								},
							},
						},
					},
				},
			},
			{
				Index:            1,
				UserText:         "continue",
				CompactionBefore: &model.Compaction{Trigger: "manual", PreTokens: 1000},
				Responses: []model.AssistantResponse{
					{Model: "claude-opus-4-8", TextBlocks: []model.TextBlock{{Text: "Done."}}},
				},
			},
		},
	}
}

func goldenCheck(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (run with -update): %v", path, err)
	}
	if string(got) != string(want) {
		t.Errorf("output mismatch for %s; run `go test ./internal/render -update` to refresh.\n--- got ---\n%s", name, got)
	}
}

func mainOutput(t *testing.T, outs []Output, ext string) []byte {
	t.Helper()
	for _, o := range outs {
		if filepath.Ext(o.RelPath) == ext {
			return o.Bytes
		}
	}
	t.Fatalf("no %s output found", ext)
	return nil
}

func TestMarkdownGolden(t *testing.T) {
	outs, err := Markdown{}.Render(sampleSession())
	if err != nil {
		t.Fatal(err)
	}
	goldenCheck(t, "sample.md", mainOutput(t, outs, ".md"))
}

func TestJSONGolden(t *testing.T) {
	outs, err := JSON{}.Render(sampleSession())
	if err != nil {
		t.Fatal(err)
	}
	goldenCheck(t, "sample.json", mainOutput(t, outs, ".json"))
}

func TestImageAssetExtracted(t *testing.T) {
	outs, err := Markdown{}.Render(sampleSession())
	if err != nil {
		t.Fatal(err)
	}
	var foundPNG bool
	for _, o := range outs {
		if filepath.Ext(o.RelPath) == ".png" {
			foundPNG = true
			if string(o.Bytes) != "PNGDATA" {
				t.Errorf("png asset bytes = %q", o.Bytes)
			}
			if filepath.Dir(o.RelPath) != "sess-123.assets" {
				t.Errorf("asset dir = %q, want sess-123.assets", filepath.Dir(o.RelPath))
			}
		}
	}
	if !foundPNG {
		t.Error("expected an extracted .png asset")
	}
}

func TestSubagentStem(t *testing.T) {
	sub := &model.Session{IsSubagent: true, AgentID: "abc123", ParentSessionID: "sess-123"}
	if stem := OutputStem(sub); stem != "sess-123.agent-abc123" {
		t.Errorf("subagent stem = %q", stem)
	}
}
