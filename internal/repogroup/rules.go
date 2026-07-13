package repogroup

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Rule is a user-supplied grouping rule: sessions whose subject (working
// directory, or project key when no cwd was recorded) matches Pattern are placed
// in Group. Group may reference regex capture groups with $1, ${name}, etc.
type Rule struct {
	Pattern string `json:"pattern"`
	Group   string `json:"group"`
}

type compiledRule struct {
	re      *regexp.Regexp
	group   string
	pattern string
}

// RuleSet is an ordered list of grouping rules; the first match wins.
type RuleSet struct {
	rules []compiledRule
}

// rulesFile is the on-disk schema for a rules file.
type rulesFile struct {
	Rules []Rule `json:"rules"`
}

// LoadRules reads and compiles a JSON rules file:
//
//	{"rules": [ {"pattern": "armand[-.]dev", "group": "github.com/me/armand.dev"}, ... ]}
func LoadRules(path string) (*RuleSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rf rulesFile
	if err := json.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return CompileRules(rf.Rules)
}

// CompileRules compiles a slice of rules, reporting the first invalid pattern.
func CompileRules(rules []Rule) (*RuleSet, error) {
	rs := &RuleSet{}
	for i, r := range rules {
		if strings.TrimSpace(r.Pattern) == "" || strings.TrimSpace(r.Group) == "" {
			return nil, fmt.Errorf("rule %d: pattern and group are both required", i)
		}
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return nil, fmt.Errorf("rule %d (%q): %w", i, r.Pattern, err)
		}
		rs.rules = append(rs.rules, compiledRule{re: re, group: r.Group, pattern: r.Pattern})
	}
	return rs, nil
}

// Empty reports whether the rule set has no rules.
func (rs *RuleSet) Empty() bool { return rs == nil || len(rs.rules) == 0 }

// Apply returns the group for the first rule matching subject, expanding capture
// references in the group template and sanitizing the result into a safe path.
// pattern is the matched rule's source, for reporting. ok is false if nothing
// matched.
func (rs *RuleSet) Apply(subject string) (group, pattern string, ok bool) {
	if rs == nil {
		return "", "", false
	}
	for _, r := range rs.rules {
		loc := r.re.FindStringSubmatchIndex(subject)
		if loc == nil {
			continue
		}
		expanded := string(r.re.ExpandString(nil, r.group, subject, loc))
		if g := sanitizePath(expanded); g != "" {
			return g, r.pattern, true
		}
	}
	return "", "", false
}

// sanitizePath makes a "/"-joined group template safe as a relative output path:
// each segment is sanitized and empty/traversal segments are dropped.
func sanitizePath(s string) string {
	var segs []string
	for _, seg := range strings.Split(s, "/") {
		if seg = sanitizeSegment(seg); seg != "" {
			segs = append(segs, seg)
		}
	}
	return strings.Join(segs, "/")
}
