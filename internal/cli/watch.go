package cli

import (
	"fmt"
	"os/signal"
	"syscall"

	"github.com/armandmcqueen/ccsessions/internal/pipeline"
	"github.com/armandmcqueen/ccsessions/internal/render"
	"github.com/armandmcqueen/ccsessions/internal/watch"
	"github.com/spf13/cobra"
)

func newWatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch the Claude home and re-render sessions in real time",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := cfgFromCmd(cmd)
			if err != nil {
				return err
			}
			reg := render.DefaultRegistry()
			renderers, unknown := reg.Resolve(cfg.Formats)
			if len(unknown) > 0 {
				return fmt.Errorf("unknown format(s) %v; available: %v", unknown, reg.Names())
			}

			opts := pipeline.Options{
				ClaudeDir: cfg.ClaudeDir,
				OutDir:    cfg.OutDir,
				Renderers: renderers,
				GroupBy:   cfg.GroupBy,
				Logf: func(format string, a ...any) {
					fmt.Fprintf(cmd.ErrOrStderr(), format+"\n", a...)
				},
			}

			debounce := cfg.Debounce
			if debounce <= 0 {
				debounce = defaultDebounce
			}
			w, err := watch.New(opts, cfg.Projects, debounce)
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			forceInitial, _ := cmd.Flags().GetBool("force-initial")
			fmt.Fprintf(cmd.OutOrStdout(), "ccsessions watch → %s (debounce %s); Ctrl-C to stop\n", opts.OutDir, debounce)
			return w.Run(ctx, forceInitial)
		},
	}
	addOutputFlags(cmd)
	cmd.Flags().Duration("debounce", defaultDebounce, "coalesce rapid changes within this window")
	cmd.Flags().Bool("force-initial", false, "do a full re-render on startup before watching")
	return cmd
}
