// Package render turns a parsed model.Session into output files. Renderers are
// pure (Session -> []Output); the pipeline owns all filesystem I/O.
package render

import (
	"sort"

	"github.com/armandmcqueen/ccsessions/internal/model"
)

// Output is one file produced by a renderer. RelPath is relative to the session's
// project output directory (<out>/<project_key>/).
type Output struct {
	RelPath string
	Bytes   []byte
}

// Renderer converts a session into one or more output files.
type Renderer interface {
	// Name is the renderer's key, used by --format and as the registry key.
	Name() string
	// MainExt is the extension (with dot) of the renderer's main document, used
	// by the pipeline to locate outputs for incremental rendering.
	MainExt() string
	// Render produces the main document plus any side artifacts (e.g. images).
	Render(s *model.Session) ([]Output, error)
}

// Registry maps renderer names to implementations.
type Registry struct {
	m map[string]Renderer
}

// NewRegistry builds a registry from the given renderers.
func NewRegistry(rs ...Renderer) *Registry {
	reg := &Registry{m: make(map[string]Renderer, len(rs))}
	for _, r := range rs {
		reg.m[r.Name()] = r
	}
	return reg
}

// DefaultRegistry returns the registry with all built-in renderers.
func DefaultRegistry() *Registry {
	return NewRegistry(Markdown{}, JSON{})
}

// Get returns the renderer registered under name.
func (r *Registry) Get(name string) (Renderer, bool) {
	rr, ok := r.m[name]
	return rr, ok
}

// Names returns the registered renderer names, sorted.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.m))
	for n := range r.m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Resolve expands a list of requested format names. The special value "all"
// expands to every registered renderer. Unknown names are returned in the second
// result so callers can report them.
func (r *Registry) Resolve(formats []string) (renderers []Renderer, unknown []string) {
	for _, f := range formats {
		if f == "all" {
			return r.all(), nil
		}
		if rr, ok := r.m[f]; ok {
			renderers = append(renderers, rr)
		} else {
			unknown = append(unknown, f)
		}
	}
	return renderers, unknown
}

func (r *Registry) all() []Renderer {
	out := make([]Renderer, 0, len(r.m))
	for _, n := range r.Names() {
		out = append(out, r.m[n])
	}
	return out
}

// OutputStem is the filename stem (no extension) for a session's main document.
// Top-level sessions use their session id; subagents are named relative to the
// root session: <root_session_id>.agent-<agent_id>.
func OutputStem(s *model.Session) string {
	if s.IsSubagent {
		return s.ParentSessionID + ".agent-" + s.AgentID
	}
	return s.SessionID
}

// rootSessionID returns the top-level session id for link targets. For a subagent
// it is the parent (root) session id; for a root session it is its own id.
func rootSessionID(s *model.Session) string {
	if s.IsSubagent {
		return s.ParentSessionID
	}
	return s.SessionID
}
