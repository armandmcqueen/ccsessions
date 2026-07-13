package repogroup

import "testing"

func mustRules(t *testing.T, rules ...Rule) *RuleSet {
	t.Helper()
	rs, err := CompileRules(rules)
	if err != nil {
		t.Fatal(err)
	}
	return rs
}

func TestRulesFirstMatchWins(t *testing.T) {
	rs := mustRules(t,
		Rule{Pattern: "armand[-.]dev-workdirs", Group: "github.com/me/armand.dev-workdirs"},
		Rule{Pattern: "armand[-.]dev", Group: "github.com/me/armand.dev"},
	)
	cases := map[string]string{
		"/Users/me/code/workdirs/foo/armand.dev":                "github.com/me/armand.dev",
		"/Users/me/code/armand.dev-workdirs":                    "github.com/me/armand.dev-workdirs",
		"-Users-me-code-workdirs-foo-armand-dev--claude-wt-bar": "github.com/me/armand.dev", // dash-encoded project key
		"/Users/me/code/other":                                  "",                         // no match
	}
	for subject, want := range cases {
		got, _, ok := rs.Apply(subject)
		if want == "" {
			if ok {
				t.Errorf("Apply(%q) matched %q, want no match", subject, got)
			}
			continue
		}
		if !ok || got != want {
			t.Errorf("Apply(%q) = (%q,%v), want %q", subject, got, ok, want)
		}
	}
}

func TestRulesCaptureExpansion(t *testing.T) {
	rs := mustRules(t, Rule{Pattern: `github\.com[:/]([^/]+)/([^/.]+)`, Group: "github.com/$1/$2"})
	got, _, ok := rs.Apply("git@github.com:acme/tool.git")
	if !ok || got != "github.com/acme/tool" {
		t.Fatalf("Apply = (%q,%v), want github.com/acme/tool", got, ok)
	}
}

func TestRulesSanitizeTraversal(t *testing.T) {
	rs := mustRules(t, Rule{Pattern: "x", Group: "../../etc/passwd"})
	got, _, ok := rs.Apply("x")
	if !ok || got != "etc/passwd" {
		t.Fatalf("Apply = %q, want traversal stripped to etc/passwd", got)
	}
}

func TestRulesOverrideGitInGrouper(t *testing.T) {
	g := New(ModeRepo)
	// Even though this cwd would git-resolve, the rule wins.
	g.resolve = func(string) resolution {
		return resolution{key: "github.com/me/wrong", reason: ReasonGitRemote, ok: true}
	}
	g.SetRules(mustRules(t, Rule{Pattern: "armand[-.]dev", Group: "github.com/me/armand.dev"}))

	key, reason, _ := g.Explain("/code/armand.dev", "-proj")
	if key != "github.com/me/armand.dev" || reason != ReasonRule {
		t.Fatalf("Explain = (%q,%q), want rule override", key, reason)
	}
}

func TestRulesMatchProjectKeyWhenNoCwd(t *testing.T) {
	g := New(ModeRepo)
	g.SetRules(mustRules(t, Rule{Pattern: "armand[-.]dev", Group: "github.com/me/armand.dev"}))
	// No cwd → subject is the project key (dash-encoded).
	key, reason, _ := g.Explain("", "-Users-me-code-workdirs-x-armand-dev--claude-worktrees-y")
	if key != "github.com/me/armand.dev" || reason != ReasonRule {
		t.Fatalf("Explain = (%q,%q), want rule match on project key", key, reason)
	}
}

func TestCompileRulesRejectsBadPattern(t *testing.T) {
	if _, err := CompileRules([]Rule{{Pattern: "(", Group: "x"}}); err == nil {
		t.Fatal("expected error for invalid regex")
	}
}
