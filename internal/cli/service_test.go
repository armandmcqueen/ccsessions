package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/armandmcqueen/ccsessions/internal/config"
)

func TestServiceConfigBakesArguments(t *testing.T) {
	cfg := config.Config{
		ClaudeDir: "/home/u/.claude",
		OutDir:    "/home/u/data/ccsessions",
		Formats:   []string{"markdown", "json"},
		Projects:  []string{"proj-a", "proj-b"},
		Debounce:  750 * time.Millisecond,
	}
	sc := serviceConfig(cfg)

	if sc.Name != serviceName {
		t.Errorf("Name = %q, want %q", sc.Name, serviceName)
	}

	args := strings.Join(sc.Arguments, " ")
	for _, want := range []string{
		"service run",
		"--out /home/u/data/ccsessions",
		"--claude-dir /home/u/.claude",
		"--debounce 750ms",
		"--format markdown,json",
		"--project proj-a",
		"--project proj-b",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("baked arguments missing %q\n  got: %s", want, args)
		}
	}

	// Per-user, auto-start, auto-restart.
	for _, key := range []string{"UserService", "RunAtLoad", "KeepAlive"} {
		if v, ok := sc.Option[key]; !ok || v != true {
			t.Errorf("Option[%q] = %v (ok=%v), want true", key, v, ok)
		}
	}
}

func TestServiceLogPathNonEmpty(t *testing.T) {
	if p := serviceLogPath(); p == "" || !strings.HasSuffix(p, ".log") {
		t.Errorf("serviceLogPath() = %q, want a .log path", p)
	}
}
