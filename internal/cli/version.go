package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Build metadata, injected via -ldflags at release time by goreleaser.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "ccsessions %s (commit %s, built %s)\n", version, commit, date)
			return err
		},
	}
}
