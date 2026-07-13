package repogroup

import "testing"

func TestNormalizeRemote(t *testing.T) {
	cases := map[string]string{
		"git@github.com:armandmcqueen/armand.dev.git":       "github.com/armandmcqueen/armand.dev",
		"https://github.com/armandmcqueen/armand.dev.git":   "github.com/armandmcqueen/armand.dev",
		"https://github.com/armandmcqueen/armand.dev":       "github.com/armandmcqueen/armand.dev",
		"ssh://git@github.com/armandmcqueen/armand.dev.git": "github.com/armandmcqueen/armand.dev",
		"git@gitlab.com:group/sub/proj.git":                 "gitlab.com/group/sub/proj",
		"":                                                  "",
		"not a url":                                         "",
	}
	for in, want := range cases {
		if got := normalizeRemote(in); got != want {
			t.Errorf("normalizeRemote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSSHAndHTTPSCollapse(t *testing.T) {
	ssh := normalizeRemote("git@github.com:me/repo.git")
	https := normalizeRemote("https://github.com/me/repo.git")
	if ssh != https || ssh == "" {
		t.Errorf("ssh %q and https %q should collapse to the same non-empty key", ssh, https)
	}
}

func TestSanitizeSegmentTraversal(t *testing.T) {
	for _, bad := range []string{"..", ".", "", "a/b", "a\\b"} {
		got := sanitizeSegment(bad)
		if got == ".." || got == "." || got == "a/b" || got == "a\\b" {
			t.Errorf("sanitizeSegment(%q) = %q leaked a separator or traversal", bad, got)
		}
	}
}

func TestProjectModePassesThrough(t *testing.T) {
	g := New(ModeProject)
	if got := g.Key("/some/cwd", "-proj-key"); got != "-proj-key" {
		t.Errorf("project mode Key = %q, want -proj-key", got)
	}
}

// TestDeletedDirMergesViaBasename verifies the core requirement: a session whose
// working directory no longer exists is grouped with living worktrees of the same
// repo, via the basename index primed from the live sibling.
func TestDeletedDirMergesViaBasename(t *testing.T) {
	g := New(ModeRepo)
	// Injected resolver: only the "live" worktree resolves; the deleted one fails.
	g.resolve = func(cwd string) (string, bool) {
		if cwd == "/code/workdirs/live/armand.dev" {
			return "github.com/me/armand.dev", true
		}
		return "", false // deleted / not a repo
	}

	// Prime with the live sibling first (as RenderAll does).
	g.Prime("/code/workdirs/live/armand.dev")

	live := g.Key("/code/workdirs/live/armand.dev", "-proj-live")
	dead := g.Key("/code/workdirs/gone/armand.dev", "-proj-dead")

	if live != "github.com/me/armand.dev" {
		t.Errorf("live key = %q", live)
	}
	if dead != live {
		t.Errorf("deleted-dir key = %q, want it to merge with live sibling %q", dead, live)
	}
}

func TestUnresolvableFallsBackToBasename(t *testing.T) {
	g := New(ModeRepo)
	g.resolve = func(string) (string, bool) { return "", false }
	// No live sibling primed this basename, so it groups by the sanitized basename.
	if got := g.Key("/code/gone/mystery", "-proj"); got != "mystery" {
		t.Errorf("Key = %q, want basename fallback \"mystery\"", got)
	}
}

func TestEmptyCwdUsesProjectKey(t *testing.T) {
	g := New(ModeRepo)
	if got := g.Key("", "-proj-key"); got != "-proj-key" {
		t.Errorf("empty cwd Key = %q, want project key", got)
	}
}
