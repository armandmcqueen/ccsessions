package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/armandmcqueen/ccsessions/internal/model"
)

// Markdown renders a session as a readable markdown transcript plus extracted
// image assets.
type Markdown struct{}

func (Markdown) Name() string { return "markdown" }

func (Markdown) MainExt() string { return ".md" }

func (Markdown) Render(s *model.Session) ([]Output, error) {
	stem := OutputStem(s)
	assets := assignImageRefs(stem, s)

	var b strings.Builder
	writeHeader(&b, s)
	for i := range s.Turns {
		writeTurn(&b, s, &s.Turns[i])
	}

	outputs := make([]Output, 0, 1+len(assets))
	outputs = append(outputs, Output{RelPath: stem + ".md", Bytes: []byte(b.String())})
	outputs = append(outputs, assets...)
	return outputs, nil
}

func writeHeader(b *strings.Builder, s *model.Session) {
	title := s.Meta.Title
	if s.IsSubagent {
		name := s.Meta.AgentName
		if name == "" {
			name = s.AgentID
		}
		fmt.Fprintf(b, "# Subagent: %s\n\n", name)
		fmt.Fprintf(b, "← [Back to parent session](%s.md)\n\n", s.ParentSessionID)
	} else {
		if title == "" {
			title = s.SessionID
		}
		fmt.Fprintf(b, "# %s\n\n", title)
	}

	fmt.Fprintf(b, "- **Session:** `%s`\n", s.SessionID)
	fmt.Fprintf(b, "- **Project:** `%s`\n", s.ProjectKey)
	if s.GitBranch != "" {
		fmt.Fprintf(b, "- **Branch:** `%s`\n", s.GitBranch)
	}
	if s.Meta.AgentName != "" && !s.IsSubagent {
		fmt.Fprintf(b, "- **Agent:** %s\n", s.Meta.AgentName)
	}
	if s.Meta.PRURL != "" {
		fmt.Fprintf(b, "- **PR:** [#%d](%s)\n", s.Meta.PRNumber, s.Meta.PRURL)
	}
	if s.Meta.ForkRef != "" {
		fmt.Fprintf(b, "- **Forked from:** `%s` (parent context not included)\n", s.Meta.ForkRef)
	}
	if s.Version != "" {
		fmt.Fprintf(b, "- **Claude Code version:** %s\n", s.Version)
	}
	fmt.Fprintf(b, "- **Turns:** %d\n", len(s.Turns))
	b.WriteString("\n")
}

func writeTurn(b *strings.Builder, s *model.Session, t *model.Turn) {
	if t.CompactionBefore != nil {
		c := t.CompactionBefore
		fmt.Fprintf(b, "> 🗜️ **Context compacted** (%s", c.Trigger)
		if c.PreTokens > 0 {
			fmt.Fprintf(b, ", %d tokens before", c.PreTokens)
		}
		b.WriteString(")\n\n")
	}

	b.WriteString("---\n\n")
	header := fmt.Sprintf("## Turn %d", t.Index+1)
	if !t.Timestamp.IsZero() {
		header += " · " + t.Timestamp.Format(time.RFC3339)
	}
	if t.DurationMs > 0 {
		header += fmt.Sprintf(" · %s", time.Duration(t.DurationMs)*time.Millisecond)
	}
	b.WriteString(header + "\n\n")

	if t.UserText != "" {
		b.WriteString("### 👤 User\n\n")
		writeQuoted(b, t.UserText)
		b.WriteString("\n")
	}

	for ri := range t.Responses {
		writeResponse(b, s, &t.Responses[ri])
	}
}

func writeResponse(b *strings.Builder, s *model.Session, r *model.AssistantResponse) {
	heading := "### 🤖 Assistant"
	if r.Model != "" {
		heading += " (" + r.Model + ")"
	}
	b.WriteString(heading + "\n\n")

	if len(r.ThinkingBlocks) > 0 {
		fmt.Fprintf(b, "> 💭 *%d thinking block(s) (content redacted by Claude Code)*\n\n", len(r.ThinkingBlocks))
	}
	for _, tb := range r.TextBlocks {
		if strings.TrimSpace(tb.Text) != "" {
			b.WriteString(tb.Text + "\n\n")
		}
	}
	for ci := range r.ToolCalls {
		writeToolCall(b, s, &r.ToolCalls[ci])
	}
}

func writeToolCall(b *strings.Builder, s *model.Session, c *model.ToolCall) {
	fmt.Fprintf(b, "#### 🔧 %s\n\n", c.Name)

	if input := prettyJSON(c.Input); input != "" {
		b.WriteString("Input:\n\n")
		writeFenced(b, "json", input)
	}

	if c.SubagentID != "" {
		target := rootSessionID(s) + ".agent-" + c.SubagentID + ".md"
		fmt.Fprintf(b, "→ **Subagent transcript:** [%s](%s)\n\n", target, target)
	}

	if c.Result != "" {
		label := "Result:"
		if c.IsError {
			label = "Result (error):"
		}
		b.WriteString(label + "\n\n")
		writeFenced(b, "", c.Result)
	}

	for _, img := range c.ResultImages {
		if img.Ref != "" {
			fmt.Fprintf(b, "![image](%s)\n\n", img.Ref)
		}
	}
}

// prettyJSON indents raw json; returns "" for empty input.
func prettyJSON(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return string(raw)
	}
	return out.String()
}

// writeQuoted writes text as a markdown blockquote, preserving line breaks.
func writeQuoted(b *strings.Builder, text string) {
	for _, ln := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		b.WriteString("> " + ln + "\n")
	}
	b.WriteString("\n")
}

// writeFenced writes content in a fenced code block, choosing a fence long enough
// to safely contain any backtick runs inside the content (full fidelity).
func writeFenced(b *strings.Builder, lang, content string) {
	fence := longestFence(content)
	b.WriteString(fence + lang + "\n")
	b.WriteString(content)
	if !strings.HasSuffix(content, "\n") {
		b.WriteString("\n")
	}
	b.WriteString(fence + "\n\n")
}

// longestFence returns a backtick fence at least 3 long and longer than any run
// of backticks in content, so fenced blocks never break on embedded backticks.
func longestFence(content string) string {
	longest := 0
	run := 0
	for _, r := range content {
		if r == '`' {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	n := 3
	if longest+1 > n {
		n = longest + 1
	}
	return strings.Repeat("`", n)
}
