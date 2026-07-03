package cli

import (
	"bytes"
	"strings"
	"testing"
)

// run executes the root command with args and captures stdout.
func run(t *testing.T, args ...string) string {
	t.Helper()
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute(%v) error: %v", args, err)
	}
	return out.String()
}

func TestVersionCmd(t *testing.T) {
	got := run(t, "version")
	if !strings.HasPrefix(got, "ccsessions ") {
		t.Errorf("version output = %q, want prefix %q", got, "ccsessions ")
	}
}

func TestHelpListsCommands(t *testing.T) {
	got := run(t, "--help")
	for _, sub := range []string{"render", "watch", "list", "version"} {
		if !strings.Contains(got, sub) {
			t.Errorf("help output missing subcommand %q\n%s", sub, got)
		}
	}
}

func TestRenderEmptyClaudeDir(t *testing.T) {
	// Isolated empty claude-dir so the test never touches the real ~/.claude.
	src := t.TempDir()
	out := t.TempDir()
	got := run(t, "render", "--claude-dir", src, "--out", out, "--format", "json")
	if !strings.Contains(got, "0 sessions") {
		t.Errorf("expected 0 sessions for empty claude-dir, got: %q", got)
	}
	if !strings.Contains(got, out) {
		t.Errorf("render did not report --out %q: %q", out, got)
	}
}

func TestRenderUnknownFormat(t *testing.T) {
	src := t.TempDir()
	root := newRootCmd()
	root.SetArgs([]string{"render", "--claude-dir", src, "--format", "bogus"})
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for unknown format")
	}
}
