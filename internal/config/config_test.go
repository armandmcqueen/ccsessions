package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

// newFlags mirrors the persistent flags registered by the CLI root, so config
// resolution can be tested without importing the cli package.
func newFlags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("claude-dir", DefaultClaudeDir, "")
	fs.String("out", DefaultOut, "")
	fs.String("format", DefaultFormat, "")
	fs.StringSlice("project", nil, "")
	fs.Bool("verbose", false, "")
	fs.Bool("quiet", false, "")
	fs.Duration("debounce", 500*time.Millisecond, "")
	fs.Bool("force", false, "")
	return fs
}

func TestResolvePrecedence(t *testing.T) {
	home, _ := os.UserHomeDir()

	t.Run("defaults expand home", func(t *testing.T) {
		fs := newFlags()
		if err := fs.Parse(nil); err != nil {
			t.Fatal(err)
		}
		cfg, err := Resolve(fs)
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(home, ".claude"); cfg.ClaudeDir != want {
			t.Errorf("ClaudeDir = %q, want %q", cfg.ClaudeDir, want)
		}
		if want := filepath.Join(home, ".ai/claude-sessions"); cfg.OutDir != want {
			t.Errorf("OutDir = %q, want %q", cfg.OutDir, want)
		}
		if len(cfg.Formats) != 2 || cfg.Formats[0] != "markdown" || cfg.Formats[1] != "json" {
			t.Errorf("Formats = %v, want [markdown json]", cfg.Formats)
		}
	})

	t.Run("env overrides default", func(t *testing.T) {
		t.Setenv(EnvOut, "/tmp/envout")
		fs := newFlags()
		if err := fs.Parse(nil); err != nil {
			t.Fatal(err)
		}
		cfg, err := Resolve(fs)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.OutDir != "/tmp/envout" {
			t.Errorf("OutDir = %q, want /tmp/envout", cfg.OutDir)
		}
	})

	t.Run("flag overrides env", func(t *testing.T) {
		t.Setenv(EnvOut, "/tmp/envout")
		fs := newFlags()
		if err := fs.Parse([]string{"--out", "/tmp/flagout"}); err != nil {
			t.Fatal(err)
		}
		cfg, err := Resolve(fs)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.OutDir != "/tmp/flagout" {
			t.Errorf("OutDir = %q, want /tmp/flagout", cfg.OutDir)
		}
	})
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	cases := map[string]string{
		"~":         home,
		"~/.claude": filepath.Join(home, ".claude"),
	}
	for in, want := range cases {
		got, err := expandHome(in)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("expandHome(%q) = %q, want %q", in, got, want)
		}
	}
}
