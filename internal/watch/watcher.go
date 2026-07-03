// Package watch implements the real-time daemon: it watches a Claude home for
// session changes and re-renders affected sessions, debounced per session.
package watch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/armandmcqueen/ccsessions/internal/discover"
	"github.com/armandmcqueen/ccsessions/internal/pipeline"
	"github.com/fsnotify/fsnotify"
)

// Watcher re-renders sessions as their source files change.
type Watcher struct {
	opts     pipeline.Options
	filters  []string
	debounce time.Duration

	fsw    *fsnotify.Watcher
	mu     sync.Mutex
	timers map[string]*time.Timer // sessionKey -> pending debounce timer
	jobs   chan discover.SessionRef
}

// New creates a Watcher. opts.Force is applied to per-change renders; the startup
// catch-up pass uses incremental semantics regardless.
func New(opts pipeline.Options, filters []string, debounce time.Duration) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		opts:     opts,
		filters:  filters,
		debounce: debounce,
		fsw:      fsw,
		timers:   make(map[string]*time.Timer),
		jobs:     make(chan discover.SessionRef, 256),
	}, nil
}

func (w *Watcher) logf(format string, args ...any) {
	if w.opts.Logf != nil {
		w.opts.Logf(format, args...)
	}
}

// Run does an initial catch-up render, then watches until ctx is cancelled.
// forceInitial makes the catch-up pass re-render everything.
func (w *Watcher) Run(ctx context.Context, forceInitial bool) error {
	defer w.fsw.Close()

	// Worker drains render jobs serially (renders are idempotent and atomic).
	go w.worker(ctx)

	// Startup catch-up pass for changes that happened while the daemon was down.
	startup := w.opts
	startup.Force = forceInitial
	st, err := pipeline.RenderAll(startup, w.filters)
	if err != nil {
		return err
	}
	w.logf("startup: %d sessions, %d rendered, %d up-to-date", st.Sessions, st.Rendered, st.Skipped)

	// Watch the projects tree (recursively, with dynamic dir additions).
	if err := w.addTree(discover.ProjectsDir(w.opts.ClaudeDir)); err != nil {
		return err
	}
	w.logf("watching %s", discover.ProjectsDir(w.opts.ClaudeDir))

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-w.fsw.Events:
			if !ok {
				return nil
			}
			w.handleEvent(event)
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return nil
			}
			w.logf("watch error: %v", err)
		}
	}
}

// handleEvent reacts to a filesystem event: new directories get watched (so new
// projects and subagent dirs are covered), and changed .jsonl files schedule a
// debounced re-render of their session.
func (w *Watcher) handleEvent(event fsnotify.Event) {
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename) == 0 {
		return
	}
	if event.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			// A new project or session/subagents dir appeared; watch it and
			// reconcile any files created before the watch was added.
			if err := w.addTree(event.Name); err != nil {
				w.logf("watch new dir %s: %v", event.Name, err)
			}
			w.reconcile(event.Name)
			return
		}
	}
	if ref, ok := discover.RefFromPath(w.opts.ClaudeDir, event.Name); ok {
		w.schedule(ref)
	}
}

// reconcile schedules renders for any session files already present in a freshly
// created directory (covers the race between dir creation and adding its watch).
func (w *Watcher) reconcile(dir string) {
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if ref, ok := discover.RefFromPath(w.opts.ClaudeDir, path); ok {
			w.schedule(ref)
		}
		return nil
	})
}

// schedule coalesces rapid changes to a session into a single render after the
// debounce window elapses.
func (w *Watcher) schedule(ref discover.SessionRef) {
	if !matchFilters(ref.ProjectKey, w.filters) {
		return
	}
	key := ref.ProjectKey + "/" + ref.SessionID
	w.mu.Lock()
	defer w.mu.Unlock()
	if t, ok := w.timers[key]; ok {
		t.Reset(w.debounce)
		return
	}
	w.timers[key] = time.AfterFunc(w.debounce, func() {
		w.mu.Lock()
		delete(w.timers, key)
		w.mu.Unlock()
		select {
		case w.jobs <- ref:
		default:
			w.logf("job queue full, dropping %s (will catch up on next change)", key)
		}
	})
}

// worker renders queued sessions one at a time.
func (w *Watcher) worker(ctx context.Context) {
	jobOpts := w.opts
	jobOpts.Force = true // a change event means the session is known-stale
	for {
		select {
		case <-ctx.Done():
			return
		case ref := <-w.jobs:
			_, files, err := pipeline.RenderOne(jobOpts, ref)
			if err != nil {
				w.logf("render %s: %v", ref.SessionID, err)
				continue
			}
			w.logf("rendered %s (%d files)", ref.SessionID, files)
		}
	}
}

// addTree adds a watch on dir and every subdirectory beneath it.
func (w *Watcher) addTree(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable dirs rather than abort the whole walk
		}
		if d.IsDir() {
			if addErr := w.fsw.Add(path); addErr != nil {
				w.logf("add watch %s: %v", path, addErr)
			}
		}
		return nil
	})
}

func matchFilters(key string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	lower := strings.ToLower(key)
	for _, f := range filters {
		if f == "" || strings.Contains(lower, strings.ToLower(f)) {
			return true
		}
	}
	return false
}
