package discover

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPeekCWDFindsCwdAfterManyMetadataLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.jsonl")

	var b strings.Builder
	// 30 leading metadata lines with no cwd (mimics ai-title, mode, and a run of
	// file-history-snapshot records), then a real entry carrying the cwd.
	for i := 0; i < 30; i++ {
		b.WriteString(`{"type":"file-history-snapshot","messageId":"m"}` + "\n")
	}
	b.WriteString(`{"type":"user","cwd":"/Users/me/code/armand.dev","message":{"role":"user","content":"hi"}}` + "\n")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := PeekCWD(path); got != "/Users/me/code/armand.dev" {
		t.Errorf("PeekCWD = %q, want the cwd from line 31", got)
	}
}

func TestPeekCWDHandlesHugeLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.jsonl")
	// A ~5MB first line (large pasted attachment) must not break the scan, and
	// the cwd on the next line must still be found.
	huge := `{"type":"user","message":{"content":"` + strings.Repeat("x", 5<<20) + `"}}`
	doc := huge + "\n" + `{"type":"assistant","cwd":"/repo/here","message":{}}` + "\n"
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := PeekCWD(path); got != "/repo/here" {
		t.Errorf("PeekCWD = %q, want /repo/here", got)
	}
}

func TestExtractCWD(t *testing.T) {
	cases := map[string]string{
		`{"cwd":"/a/b","x":1}`: "/a/b",
		`{"x":1,"cwd":"/c"}`:   "/c",
		`{"no":"cwd"}`:         "",
		`{"cwd":""}`:           "",
	}
	for in, want := range cases {
		if got := extractCWD([]byte(in)); got != want {
			t.Errorf("extractCWD(%q) = %q, want %q", in, got, want)
		}
	}
}
