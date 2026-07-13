package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/armandmcqueen/ccsessions/internal/config"
	"github.com/armandmcqueen/ccsessions/internal/pipeline"
	"github.com/armandmcqueen/ccsessions/internal/render"
	"github.com/armandmcqueen/ccsessions/internal/watch"
	"github.com/kardianos/service"
	"github.com/spf13/cobra"
)

// serviceName is the OS service identifier (launchd label / systemd unit name).
const serviceName = "ccsessions"

func newServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage ccsessions as a background service that continuously replicates sessions",
		Long: "Install ccsessions as an OS-managed background service (launchd on macOS, " +
			"systemd on Linux) that runs `watch` continuously — auto-starting at login and " +
			"restarting on crash, so your rendered sessions stay current without a terminal open.",
	}
	cmd.AddCommand(
		newServiceInstallCmd(),
		newServiceUninstallCmd(),
		newServiceStartCmd(),
		newServiceStopCmd(),
		newServiceStatusCmd(),
		newServiceRunCmd(),
	)
	return cmd
}

// watchProgram implements service.Interface: it runs the watch loop for the
// lifetime of the service, starting and stopping cleanly on OS signals.
type watchProgram struct {
	opts     pipeline.Options
	filters  []string
	debounce time.Duration

	cancel context.CancelFunc
	done   chan struct{}
}

func (p *watchProgram) Start(s service.Service) error {
	w, err := watch.New(p.opts, p.filters, p.debounce)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.done = make(chan struct{})
	go func() {
		defer close(p.done)
		if err := w.Run(ctx, false); err != nil && p.opts.Logf != nil {
			p.opts.Logf("watch loop exited: %v", err)
		}
	}()
	return nil
}

func (p *watchProgram) Stop(s service.Service) error {
	if p.cancel != nil {
		p.cancel()
	}
	if p.done != nil {
		select {
		case <-p.done:
		case <-time.After(5 * time.Second):
		}
	}
	return nil
}

// serviceConfig builds the OS service definition, baking the resolved config into
// the launch arguments so the daemon replicates to exactly the configured target
// regardless of the environment it starts in.
func serviceConfig(cfg config.Config) *service.Config {
	args := []string{
		"service", "run",
		"--out", cfg.OutDir,
		"--claude-dir", cfg.ClaudeDir,
		"--debounce", cfg.Debounce.String(),
		"--group-by", cfg.GroupBy,
	}
	if len(cfg.Formats) > 0 {
		args = append(args, "--format", strings.Join(cfg.Formats, ","))
	}
	if cfg.GroupRules != "" {
		args = append(args, "--group-rules", cfg.GroupRules)
	}
	for _, p := range cfg.Projects {
		args = append(args, "--project", p)
	}
	return &service.Config{
		Name:        serviceName,
		DisplayName: "ccsessions session replicator",
		Description: "Continuously renders Claude Code sessions to readable files.",
		Arguments:   args,
		// Run as a per-user service (no root needed), start at login, and restart
		// if it exits unexpectedly.
		Option: service.KeyValue{
			"UserService": true,
			"RunAtLoad":   true,
			"KeepAlive":   true,
		},
	}
}

// newControlService builds a minimal service handle for lifecycle control
// (start/stop/status/uninstall). These operations identify the service by name,
// so they need neither the output flags nor renderer resolution.
func newControlService() (service.Service, error) {
	return service.New(&watchProgram{}, &service.Config{
		Name:   serviceName,
		Option: service.KeyValue{"UserService": true},
	})
}

// newService constructs the kardianos service handle for the resolved config.
func newService(cfg config.Config) (service.Service, *watchProgram, error) {
	debounce := cfg.Debounce
	if debounce <= 0 {
		debounce = defaultDebounce
	}
	prog := &watchProgram{
		opts: pipeline.Options{
			ClaudeDir:  cfg.ClaudeDir,
			OutDir:     cfg.OutDir,
			GroupBy:    cfg.GroupBy,
			GroupRules: cfg.GroupRules,
			Renderers:  []render.Renderer{}, // filled in below
		},
		filters:  cfg.Projects,
		debounce: debounce,
	}
	renderers, unknown := render.DefaultRegistry().Resolve(cfg.Formats)
	if len(unknown) > 0 {
		return nil, nil, fmt.Errorf("unknown format(s) %v; available: %v", unknown, render.DefaultRegistry().Names())
	}
	prog.opts.Renderers = renderers

	svc, err := service.New(prog, serviceConfig(cfg))
	if err != nil {
		return nil, nil, err
	}
	return svc, prog, nil
}

func newServiceInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install and start the background replication service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := cfgFromCmd(cmd)
			if err != nil {
				return err
			}
			svc, _, err := newService(cfg)
			if err != nil {
				return err
			}
			if err := svc.Install(); err != nil {
				return fmt.Errorf("install: %w", err)
			}
			if err := svc.Start(); err != nil {
				return fmt.Errorf("installed but failed to start: %w", err)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Installed and started service %q.\n", serviceName)
			fmt.Fprintf(out, "  Replicating: %s → %s\n", cfg.ClaudeDir, cfg.OutDir)
			fmt.Fprintf(out, "  Formats: %s   Debounce: %s\n", strings.Join(cfg.Formats, ","), cfg.Debounce)
			fmt.Fprintf(out, "  Logs: %s\n", serviceLogPath())
			fmt.Fprintf(out, "Check it with: ccsessions service status\n")
			return nil
		},
	}
	addServiceRunFlags(cmd)
	return cmd
}

func newServiceUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Stop and remove the background service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := newControlService()
			if err != nil {
				return err
			}
			// Best-effort stop before removal; ignore "not running" errors.
			_ = svc.Stop()
			if err := svc.Uninstall(); err != nil {
				return fmt.Errorf("uninstall: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed service %q.\n", serviceName)
			return nil
		},
	}
}

func newServiceStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the installed service",
		Args:  cobra.NoArgs,
		RunE:  serviceControl(func(s service.Service) error { return s.Start() }, "started"),
	}
}

func newServiceStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the running service (without removing it)",
		Args:  cobra.NoArgs,
		RunE:  serviceControl(func(s service.Service) error { return s.Stop() }, "stopped"),
	}
}

// serviceControl builds a RunE that applies a control action to the service.
func serviceControl(action func(service.Service) error, verb string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		svc, err := newControlService()
		if err != nil {
			return err
		}
		if err := action(svc); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Service %q %s.\n", serviceName, verb)
		return nil
	}
}

func newServiceStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether the service is installed and running",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := newControlService()
			if err != nil {
				return err
			}
			status, err := svc.Status()
			out := cmd.OutOrStdout()
			switch {
			case err == service.ErrNotInstalled:
				fmt.Fprintf(out, "not installed\n")
			case err != nil:
				return err
			case status == service.StatusRunning:
				fmt.Fprintf(out, "running\n")
			case status == service.StatusStopped:
				fmt.Fprintf(out, "installed but stopped\n")
			default:
				fmt.Fprintf(out, "unknown\n")
			}
			return nil
		},
	}
}

func newServiceRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "run",
		Short:  "Run the replication loop in the foreground (invoked by the OS service)",
		Hidden: true, // internal entrypoint; users use install/start
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := cfgFromCmd(cmd)
			if err != nil {
				return err
			}
			svc, prog, err := newService(cfg)
			if err != nil {
				return err
			}
			// Route watch logs to the service logger (syslog when managed) and to
			// a logfile for easy inspection.
			logger, err := svc.Logger(nil)
			if err == nil {
				lf := openServiceLog()
				prog.opts.Logf = func(format string, a ...any) {
					msg := fmt.Sprintf(format, a...)
					_ = logger.Info(msg)
					if lf != nil {
						fmt.Fprintf(lf, "%s\n", msg)
					}
				}
			}
			return svc.Run()
		},
	}
	addServiceRunFlags(cmd)
	return cmd
}

// addServiceRunFlags registers the flags that install/run need to define what the
// daemon replicates.
func addServiceRunFlags(cmd *cobra.Command) {
	addOutputFlags(cmd)
	cmd.Flags().Duration("debounce", defaultDebounce, "coalesce rapid changes within this window")
}

// serviceLogPath returns the path where the service writes its log: the
// conventional per-user log location on macOS, otherwise the user cache dir.
func serviceLogPath() string {
	if runtime.GOOS == "darwin" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Logs", "ccsessions.log")
		}
	}
	if cache, err := os.UserCacheDir(); err == nil {
		return filepath.Join(cache, "ccsessions", "service.log")
	}
	return filepath.Join(os.TempDir(), "ccsessions.log")
}

// openServiceLog opens (creating as needed) the service logfile, or returns nil
// if it cannot be created (logging then falls back to the system logger only).
func openServiceLog() *os.File {
	path := serviceLogPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil
	}
	return f
}
