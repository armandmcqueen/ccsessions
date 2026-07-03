package pipeline

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/armandmcqueen/ccsessions/internal/render"
)

// writeSession creates a minimal one-line session file under a fake claude home
// and returns the claude dir and project key.
func writeSession(t *testing.T, projectKey, sessionID string) string {
	t.Helper()
	claudeDir := t.TempDir()
	projDir := filepath.Join(claudeDir, "projects", projectKey)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := `{"type":"user","timestamp":"2026-06-01T00:00:00.000Z","message":{"role":"user","content":"hello"}}` + "\n"
	if err := os.WriteFile(filepath.Join(projDir, sessionID+".jsonl"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	return claudeDir
}

func TestRenderAndIncremental(t *testing.T) {
	claudeDir := writeSession(t, "-proj", "sess-1")
	outDir := t.TempDir()
	opts := Options{
		ClaudeDir: claudeDir,
		OutDir:    outDir,
		Renderers: []render.Renderer{render.Markdown{}, render.JSON{}},
	}

	// First render writes the outputs.
	st, err := RenderAll(opts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st.Rendered != 1 || st.Files != 2 {
		t.Fatalf("first render stats = %+v, want 1 rendered / 2 files", st)
	}
	mdPath := filepath.Join(outDir, "-proj", "sess-1.md")
	if _, err := os.Stat(mdPath); err != nil {
		t.Fatalf("expected %s: %v", mdPath, err)
	}

	// Second render is a no-op (outputs newer than source).
	st2, err := RenderAll(opts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st2.Rendered != 0 || st2.Skipped != 1 {
		t.Fatalf("second render stats = %+v, want 0 rendered / 1 skipped", st2)
	}

	// Touch the source newer than outputs → re-render.
	future := time.Now().Add(2 * time.Second)
	src := filepath.Join(claudeDir, "projects", "-proj", "sess-1.jsonl")
	if err := os.Chtimes(src, future, future); err != nil {
		t.Fatal(err)
	}
	st3, err := RenderAll(opts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st3.Rendered != 1 {
		t.Fatalf("after touch, stats = %+v, want 1 rendered", st3)
	}

	// Force re-renders regardless of mtime.
	opts.Force = true
	st4, err := RenderAll(opts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st4.Rendered != 1 {
		t.Fatalf("force render stats = %+v, want 1 rendered", st4)
	}
}

func TestAtomicWriteNoTempLeftovers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "file.txt")
	if err := writeFileAtomic(path, []byte("data")); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "data" {
		t.Fatalf("content = %q", got)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}
