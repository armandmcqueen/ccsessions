package rawjsonl

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
)

// DecodeLine parses a single JSONL line into an Entry.
func DecodeLine(line []byte) (*Entry, error) {
	var e Entry
	if err := json.Unmarshal(line, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// DecodeAll reads an entire JSONL stream and returns the decoded entries plus the
// total number of non-empty lines seen (so callers can report parse coverage).
//
// It uses a bufio.Reader rather than bufio.Scanner because a single line can be
// many megabytes (large tool results, base64 images) and Scanner caps tokens.
// Malformed lines are skipped and still counted toward total; a truncated final
// line (common when reading a session mid-write) is tolerated.
func DecodeAll(r io.Reader) (entries []*Entry, total int, err error) {
	br := bufio.NewReaderSize(r, 1<<20)
	for {
		line, readErr := br.ReadBytes('\n')
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) > 0 {
			total++
			if e, decErr := DecodeLine(trimmed); decErr == nil {
				entries = append(entries, e)
			}
			// else: malformed line — skip but keep counting.
		}
		if readErr != nil {
			if readErr == io.EOF {
				return entries, total, nil
			}
			return entries, total, readErr
		}
	}
}

// ParseMessage decodes the entry's message envelope.
func (e *Entry) ParseMessage() (*Message, error) {
	if len(e.Message) == 0 {
		return nil, nil
	}
	var m Message
	if err := json.Unmarshal(e.Message, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// isArray reports whether raw json is an array (first non-space byte is '[').
func isArray(raw json.RawMessage) bool {
	for _, b := range raw {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		case '[':
			return true
		default:
			return false
		}
	}
	return false
}

// isString reports whether raw json is a string literal.
func isString(raw json.RawMessage) bool {
	for _, b := range raw {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		case '"':
			return true
		default:
			return false
		}
	}
	return false
}

// Blocks decodes message content into a slice of ContentBlock. A bare string
// content is returned as a single text block.
func (m *Message) Blocks() ([]ContentBlock, error) {
	if len(m.Content) == 0 {
		return nil, nil
	}
	if isString(m.Content) {
		var s string
		if err := json.Unmarshal(m.Content, &s); err != nil {
			return nil, err
		}
		return []ContentBlock{{Type: "text", Text: s}}, nil
	}
	var blocks []ContentBlock
	if err := json.Unmarshal(m.Content, &blocks); err != nil {
		return nil, err
	}
	return blocks, nil
}

// DecodedImage is a base64-decoded image extracted from tool-result content.
type DecodedImage struct {
	MediaType string
	Data      []byte
}

// ResultContent flattens a tool_result's content (which is a string or an array
// of text/image blocks) into a single text string plus any decoded images.
// Non-text, non-image blocks are rendered as a "[type]" placeholder.
func ResultContent(raw json.RawMessage) (text string, images []DecodedImage) {
	if len(raw) == 0 {
		return "", nil
	}
	if isString(raw) {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s, nil
		}
		return "", nil
	}
	if !isArray(raw) {
		return "", nil
	}
	var blocks []ContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", nil
	}
	var buf bytes.Buffer
	for _, b := range blocks {
		switch b.Type {
		case "text":
			buf.WriteString(b.Text)
		case "image":
			if b.Source != nil {
				data, _ := base64.StdEncoding.DecodeString(b.Source.Data)
				images = append(images, DecodedImage{MediaType: b.Source.MediaType, Data: data})
				buf.WriteString("[image]")
			}
		default:
			buf.WriteString("[" + b.Type + "]")
		}
	}
	return buf.String(), images
}
