package pipeline

import (
	"os"
	"path/filepath"
	"time"

	"github.com/armandmcqueen/ccsessions/internal/discover"
	"github.com/armandmcqueen/ccsessions/internal/parser"
	"github.com/armandmcqueen/ccsessions/internal/render"
)

// needsRender reports whether a session's outputs are missing or older than its
// source files. It computes the expected main-document paths (parent + each
// subagent, for each renderer) without parsing, so the check is cheap.
func needsRender(ref discover.SessionRef, outDir string, renderers []render.Renderer) bool {
	srcMtime, ok := maxMtime(ref.SourceFiles())
	if !ok {
		return false // no readable source; nothing to do
	}

	projDir := filepath.Join(outDir, ref.ProjectKey)
	stems := []string{ref.SessionID}
	for _, sp := range ref.SubagentFiles() {
		stems = append(stems, ref.SessionID+".agent-"+parser.AgentIDFromPath(sp))
	}

	var oldestOut time.Time
	first := true
	for _, stem := range stems {
		for _, r := range renderers {
			path := filepath.Join(projDir, stem+r.MainExt())
			info, err := os.Stat(path)
			if err != nil {
				return true // a required output is missing
			}
			if first || info.ModTime().Before(oldestOut) {
				oldestOut = info.ModTime()
				first = false
			}
		}
	}
	// Render if any source is at least as new as the oldest output.
	return !srcMtime.Before(oldestOut)
}

// maxMtime returns the newest modification time among the given files.
func maxMtime(paths []string) (time.Time, bool) {
	var newest time.Time
	found := false
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if !found || info.ModTime().After(newest) {
			newest = info.ModTime()
			found = true
		}
	}
	return newest, found
}
