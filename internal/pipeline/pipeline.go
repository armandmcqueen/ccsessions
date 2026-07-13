// Package pipeline ties discovery, parsing, and rendering together: it decides
// what needs re-rendering and writes outputs atomically.
package pipeline

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/armandmcqueen/ccsessions/internal/discover"
	"github.com/armandmcqueen/ccsessions/internal/model"
	"github.com/armandmcqueen/ccsessions/internal/parser"
	"github.com/armandmcqueen/ccsessions/internal/render"
	"github.com/armandmcqueen/ccsessions/internal/repogroup"
)

// Options configures a render run.
type Options struct {
	ClaudeDir  string
	OutDir     string
	Renderers  []render.Renderer
	Force      bool
	GroupBy    string                           // "repo" (default) or "project"
	GroupRules string                           // path to a regex grouping-rules file (optional)
	Grouper    *repogroup.Grouper               // shared across a bulk run; created per-call if nil
	Logf       func(format string, args ...any) // optional verbose logger
}

// grouper returns the run's grouper, creating a transient one if none was set
// (e.g. for single-session or watch renders).
func (o Options) grouper() *repogroup.Grouper {
	if o.Grouper != nil {
		return o.Grouper
	}
	g, _ := o.newGrouper()
	return g
}

// newGrouper builds a grouper for the configured mode and loads any rules file.
func (o Options) newGrouper() (*repogroup.Grouper, error) {
	mode := o.GroupBy
	if mode == "" {
		mode = repogroup.ModeRepo
	}
	g := repogroup.New(mode)
	if o.GroupRules != "" {
		rs, err := repogroup.LoadRules(o.GroupRules)
		if err != nil {
			return g, err
		}
		g.SetRules(rs)
	}
	return g, nil
}

func (o Options) logf(format string, args ...any) {
	if o.Logf != nil {
		o.Logf(format, args...)
	}
}

// Stats summarizes a render run.
type Stats struct {
	Sessions int
	Rendered int
	Skipped  int
	Files    int
	Errors   int
}

// RenderAll renders every discovered session (subject to project filters).
func RenderAll(opts Options, projectFilters []string) (Stats, error) {
	refs, err := discover.Sessions(opts.ClaudeDir, projectFilters)
	if err != nil {
		return Stats{}, err
	}

	// Build one grouper for the whole run and prime it with every session's cwd
	// first, so deleted-directory sessions can merge with live siblings by
	// basename.
	grouper, err := opts.newGrouper()
	if err != nil {
		return Stats{}, err
	}
	opts.Grouper = grouper
	for _, ref := range refs {
		opts.Grouper.Prime(discover.PeekCWD(ref.MainPath))
	}

	var st Stats
	for _, ref := range refs {
		st.Sessions++
		rendered, files, err := RenderOne(opts, ref)
		switch {
		case err != nil:
			st.Errors++
			opts.logf("error rendering %s: %v", ref.SessionID, err)
		case rendered:
			st.Rendered++
			st.Files += files
			opts.logf("rendered %s (%d files)", ref.SessionID, files)
		default:
			st.Skipped++
		}
	}
	return st, nil
}

// RenderOne renders a single session (and its subagents). It returns whether any
// work was done (false when skipped as up-to-date) and the number of files written.
func RenderOne(opts Options, ref discover.SessionRef) (rendered bool, files int, err error) {
	// Resolve the output group (repo or project) once, before parsing, so the
	// incremental check and the render write to the same directory.
	groupKey := opts.grouper().Key(discover.PeekCWD(ref.MainPath), ref.ProjectKey)

	if !opts.Force && !needsRender(ref, opts.OutDir, groupKey, opts.Renderers) {
		return false, 0, nil
	}

	parent, err := parser.ParseFile(ref.MainPath, ref.ProjectKey)
	if err != nil {
		return false, 0, err
	}
	parent.Repo = groupKey

	var subs []*model.Session
	for _, sp := range ref.SubagentFiles() {
		sub, err := parser.ParseSubagentFile(sp, ref.ProjectKey)
		if err != nil {
			opts.logf("skip subagent %s: %v", sp, err)
			continue
		}
		sub.ParentSessionID = ref.SessionID
		sub.Repo = groupKey
		subs = append(subs, sub)
	}

	// Link the parent and every subagent against the full subagent pool so both
	// top-level and nested Agent calls resolve to transcripts.
	parser.LinkSubagents(parent, subs)
	for _, sub := range subs {
		parser.LinkSubagents(sub, subs)
	}

	projDir := filepath.Join(opts.OutDir, groupKey)
	written := make(map[string]bool)
	all := append([]*model.Session{parent}, subs...)
	for _, sess := range all {
		for _, r := range opts.Renderers {
			outs, rerr := r.Render(sess)
			if rerr != nil {
				return rendered, files, fmt.Errorf("%s renderer on %s: %w", r.Name(), sess.SessionID, rerr)
			}
			for _, out := range outs {
				path := filepath.Join(projDir, out.RelPath)
				if written[path] {
					continue // dedupe identical asset across renderers/sessions
				}
				if werr := writeFileAtomic(path, out.Bytes); werr != nil {
					return rendered, files, werr
				}
				written[path] = true
				files++
			}
		}
	}
	return true, files, nil
}

// writeFileAtomic writes data to path via a temp file + rename so a reader never
// observes a partially written file.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".ccsessions-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
