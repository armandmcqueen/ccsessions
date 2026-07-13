// Package cli wires up the cobra command tree for ccsessions.
package cli

import (
	"fmt"
	"time"

	"github.com/armandmcqueen/ccsessions/internal/config"
	"github.com/spf13/cobra"
)

// Execute runs the root command. It is the single entrypoint called by main.
func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "ccsessions",
		Short: "Render Claude Code sessions to readable markdown/json files",
		Long: "ccsessions transforms Claude Code session JSONL data into per-session " +
			"markdown and json files that can be read with normal file tools.\n\n" +
			"By default it does a one-time incremental render of every session; the " +
			"watch command keeps the rendered files current in real time.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Persistent (global) flags that genuinely apply to every subcommand: which
	// Claude home to read, how to filter projects, and logging verbosity. Output
	// flags (--out, --format) live on the render/watch commands instead.
	pf := root.PersistentFlags()
	pf.String("claude-dir", config.DefaultClaudeDir, "Claude home directory (env "+config.EnvClaudeDir+")")
	pf.StringSlice("project", nil, "only sessions whose project_key contains this substring (repeatable)")
	pf.BoolP("verbose", "v", false, "verbose logging")
	pf.BoolP("quiet", "q", false, "suppress non-error output")

	root.AddCommand(
		newVersionCmd(),
		newListCmd(),
		newRenderCmd(),
		newWatchCmd(),
		newServiceCmd(),
	)
	return root
}

// cfgFromCmd resolves the merged configuration for a command invocation and
// validates it.
func cfgFromCmd(cmd *cobra.Command) (config.Config, error) {
	cfg, err := config.Resolve(cmd.Flags())
	if err != nil {
		return cfg, err
	}
	if cmd.Flags().Lookup("group-by") != nil && cfg.GroupBy != "repo" && cfg.GroupBy != "project" {
		return cfg, fmt.Errorf("invalid --group-by %q; want \"repo\" or \"project\"", cfg.GroupBy)
	}
	return cfg, nil
}

// addOutputFlags registers the flags shared by commands that render files.
func addOutputFlags(cmd *cobra.Command) {
	cmd.Flags().String("out", config.DefaultOut, "output directory for rendered sessions (env "+config.EnvOut+")")
	cmd.Flags().String("format", config.DefaultFormat, `renderers to run, comma-separated, or "all" (env `+config.EnvFormat+")")
	cmd.Flags().String("group-by", config.DefaultGroupBy, `group output by "repo" (fold worktrees of the same git repo together) or "project" (env `+config.EnvGroupBy+")")
}

// defaultDebounce is the watch coalescing window default.
const defaultDebounce = 500 * time.Millisecond
