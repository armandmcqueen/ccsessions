package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/armandmcqueen/ccsessions/internal/discover"
	"github.com/armandmcqueen/ccsessions/internal/parser"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List discovered Claude Code sessions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := cfgFromCmd(cmd)
			if err != nil {
				return err
			}
			refs, err := discover.Sessions(cfg.ClaudeDir, cfg.Projects)
			if err != nil {
				return err
			}

			type row struct {
				Project string `json:"project_key"`
				Session string `json:"session_id"`
				Title   string `json:"title,omitempty"`
				Turns   int    `json:"turns"`
			}
			rows := make([]row, 0, len(refs))
			for _, ref := range refs {
				s, err := parser.ParseFile(ref.MainPath, ref.ProjectKey)
				if err != nil {
					continue
				}
				rows = append(rows, row{ref.ProjectKey, s.SessionID, s.Meta.Title, len(s.Turns)})
			}

			out := cmd.OutOrStdout()

			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(rows)
			}

			// Piped/non-interactive: one tab-separated line per session and
			// nothing else, so `list | wc -l` equals the session count and the
			// output is easy to cut/grep/awk.
			if !isTerminal(out) {
				for _, r := range rows {
					fmt.Fprintf(out, "%s\t%s\t%d\t%s\n", r.Session, r.Project, r.Turns, r.Title)
				}
				return nil
			}

			// Interactive: aligned table with a header and a summary footer.
			tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "SESSION\tPROJECT\tTURNS\tTITLE")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", r.Session, r.Project, r.Turns, r.Title)
			}
			tw.Flush()
			fmt.Fprintf(out, "\n%d sessions\n", len(rows))
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "machine-readable JSON output")
	return cmd
}

// isTerminal reports whether w is an interactive terminal (a character device).
// In tests and pipes w is a buffer or a regular file/pipe, so this returns false.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
