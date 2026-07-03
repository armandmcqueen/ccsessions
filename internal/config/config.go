// Package config resolves runtime configuration from flags, environment, and
// defaults (in that precedence order) and expands "~" in path values.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

// Environment variable names for the persistent flags.
const (
	EnvClaudeDir = "CCSESSIONS_CLAUDE_DIR"
	EnvOut       = "CCSESSIONS_OUT"
	EnvFormat    = "CCSESSIONS_FORMAT"
)

// Default (pre-expansion) path values.
const (
	DefaultClaudeDir = "~/.claude"
	DefaultOut       = "~/.ai/claude-sessions"
	DefaultFormat    = "markdown,json"
)

// Config is the fully-resolved runtime configuration.
type Config struct {
	ClaudeDir string        // source root (expanded absolute path)
	OutDir    string        // output root (expanded absolute path)
	Formats   []string      // renderer names, e.g. ["markdown","json"] or ["all"]
	Projects  []string      // project_key substring filters; empty = all
	Debounce  time.Duration // watch coalescing window
	Force     bool          // ignore mtime, re-render everything
	Verbose   bool
	Quiet     bool
}

// Resolve builds a Config from a flag set using flag > env > default precedence.
// A flag only wins if the user actually set it (flags.Changed); otherwise the
// environment variable is consulted, then the built-in default. This avoids a
// flag left at its default value silently clobbering an env var.
func Resolve(flags *pflag.FlagSet) (Config, error) {
	claudeDir, err := expandHome(resolveString(flags, "claude-dir", EnvClaudeDir, DefaultClaudeDir))
	if err != nil {
		return Config{}, fmt.Errorf("resolving --claude-dir: %w", err)
	}
	outDir, err := expandHome(resolveString(flags, "out", EnvOut, DefaultOut))
	if err != nil {
		return Config{}, fmt.Errorf("resolving --out: %w", err)
	}

	formats := splitCSV(resolveString(flags, "format", EnvFormat, DefaultFormat))

	projects, _ := flags.GetStringSlice("project")

	cfg := Config{
		ClaudeDir: claudeDir,
		OutDir:    outDir,
		Formats:   formats,
		Projects:  projects,
	}

	// Optional flags that not every command registers.
	if flags.Lookup("debounce") != nil {
		cfg.Debounce, _ = flags.GetDuration("debounce")
	}
	if flags.Lookup("force") != nil {
		cfg.Force, _ = flags.GetBool("force")
	}
	cfg.Verbose, _ = flags.GetBool("verbose")
	cfg.Quiet, _ = flags.GetBool("quiet")

	return cfg, nil
}

// resolveString applies flag > env > default precedence for a string flag.
func resolveString(flags *pflag.FlagSet, flagName, envName, def string) string {
	if f := flags.Lookup(flagName); f != nil && f.Changed {
		return f.Value.String()
	}
	if v, ok := os.LookupEnv(envName); ok && v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// expandHome expands a leading "~" to the user home directory and returns an
// absolute, cleaned path. Shells do not expand "~" inside env vars, so we do it.
func expandHome(p string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if p == "~" {
			p = home
		} else {
			p = filepath.Join(home, p[2:])
		}
	}
	return filepath.Abs(p)
}
