package cli

import (
	"fmt"

	"github.com/armandmcqueen/ccsessions/internal/discover"
	"github.com/armandmcqueen/ccsessions/internal/pipeline"
	"github.com/armandmcqueen/ccsessions/internal/render"
	"github.com/spf13/cobra"
)

func newRenderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "render [SESSION_ID]",
		Short: "Render sessions to readable files (bulk, or a single session)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cfgFromCmd(cmd)
			if err != nil {
				return err
			}
			reg := render.DefaultRegistry()
			renderers, unknown := reg.Resolve(cfg.Formats)
			if len(unknown) > 0 {
				return fmt.Errorf("unknown format(s) %v; available: %v", unknown, reg.Names())
			}
			if len(renderers) == 0 {
				return fmt.Errorf("no renderers selected")
			}

			opts := pipeline.Options{
				ClaudeDir:  cfg.ClaudeDir,
				OutDir:     cfg.OutDir,
				Renderers:  renderers,
				Force:      cfg.Force,
				GroupBy:    cfg.GroupBy,
				GroupRules: cfg.GroupRules,
			}
			if cfg.Verbose {
				opts.Logf = func(format string, a ...any) {
					fmt.Fprintf(cmd.ErrOrStderr(), format+"\n", a...)
				}
			}

			out := cmd.OutOrStdout()

			// Single-session render: find the ref by id across all projects.
			if len(args) == 1 {
				ref, err := findSession(cfg.ClaudeDir, cfg.Projects, args[0])
				if err != nil {
					return err
				}
				rendered, files, err := pipeline.RenderOne(opts, ref)
				if err != nil {
					return err
				}
				if rendered {
					fmt.Fprintf(out, "rendered %s → %s (%d files)\n", ref.SessionID, opts.OutDir, files)
				} else {
					fmt.Fprintf(out, "%s already up to date (use --force to re-render)\n", ref.SessionID)
				}
				return nil
			}

			st, err := pipeline.RenderAll(opts, cfg.Projects)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "%d sessions: %d rendered, %d up-to-date, %d errors (%d files written)\n  → %s\n",
				st.Sessions, st.Rendered, st.Skipped, st.Errors, st.Files, opts.OutDir)
			return nil
		},
	}
	addOutputFlags(cmd)
	cmd.Flags().Bool("force", false, "re-render even if outputs are up to date")
	return cmd
}

// findSession resolves a session id to its SessionRef, searching all projects
// (subject to project filters).
func findSession(claudeDir string, filters []string, id string) (discover.SessionRef, error) {
	refs, err := discover.Sessions(claudeDir, filters)
	if err != nil {
		return discover.SessionRef{}, err
	}
	for _, ref := range refs {
		if ref.SessionID == id {
			return ref, nil
		}
	}
	return discover.SessionRef{}, fmt.Errorf("session %q not found", id)
}
