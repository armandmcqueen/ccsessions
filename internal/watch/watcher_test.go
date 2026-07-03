package watch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/armandmcqueen/ccsessions/internal/discover"
	"github.com/armandmcqueen/ccsessions/internal/pipeline"
	"github.com/armandmcqueen/ccsessions/internal/render"
)

func TestRefFromPath(t *testing.T) {
	claude := "/home/u/.claude"
	cases := []struct {
		path    string
		wantOK  bool
		wantSID string
	}{
		{"/home/u/.claude/projects/-proj/sess-1.jsonl", true, "sess-1"},
		{"/home/u/.claude/projects/-proj/sess-1/subagents/agent-abc.jsonl", true, "sess-1"},
		{"/home/u/.claude/projects/-proj/sess-1/other/file.jsonl", false, ""},
		{"/home/u/.claude/projects/-proj/notes.txt", false, ""},
	}
	for _, c := range cases {
		ref, ok := discover.RefFromPath(claude, c.path)
		if ok != c.wantOK {
			t.Errorf("RefFromPath(%q) ok = %v, want %v", c.path, ok, c.wantOK)
			continue
		}
		if ok && ref.SessionID != c.wantSID {
			t.Errorf("RefFromPath(%q) sid = %q, want %q", c.path, ref.SessionID, c.wantSID)
		}
	}
}

// TestWatchLiveUpdate writes a session after the watcher starts and asserts the
// output appears, then appends and asserts the output updates.
func TestWatchLiveUpdate(t *testing.T) {
	claudeDir := t.TempDir()
	outDir := t.TempDir()
	projDir := filepath.Join(claudeDir, "projects", "-proj")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}

	opts := pipeline.Options{
		ClaudeDir: claudeDir,
		OutDir:    outDir,
		Renderers: []render.Renderer{render.Markdown{}},
		Logf:      func(string, ...any) {},
	}
	w, err := New(opts, nil, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx, false) }()

	// Give the watcher a moment to install watches after the startup pass.
	time.Sleep(150 * time.Millisecond)

	sessFile := filepath.Join(projDir, "sess-1.jsonl")
	line1 := `{"type":"user","timestamp":"2026-06-01T00:00:00.000Z","message":{"role":"user","content":"first question"}}` + "\n"
	if err := os.WriteFile(sessFile, []byte(line1), 0o644); err != nil {
		t.Fatal(err)
	}

	outMD := filepath.Join(outDir, "-proj", "sess-1.md")
	waitForContains(t, outMD, "first question", 3*time.Second)

	// Append a second turn; output should update.
	f, err := os.OpenFile(sessFile, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	line2 := `{"type":"user","timestamp":"2026-06-01T00:01:00.000Z","message":{"role":"user","content":"second question"}}` + "\n"
	if _, err := f.WriteString(line2); err != nil {
		t.Fatal(err)
	}
	f.Close()

	waitForContains(t, outMD, "second question", 3*time.Second)

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop after cancel")
	}
}

// waitForContains polls a file until it contains sub or the deadline passes.
func waitForContains(t *testing.T, path, sub string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil {
			if strings.Contains(string(b), sub) {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q to contain %q", path, sub)
}
