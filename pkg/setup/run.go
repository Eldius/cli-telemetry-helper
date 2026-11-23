package setup

import (
	"context"
	"log/slog"

	"github.com/spf13/cobra"
)

// Run executes rootCmd with configuration and telemetry fully wired: it
// initializes viper (see InitViper), unmarshals "telemetry.backends"
// (expanding ${VAR} references), sets up the enabled OTel providers,
// attaches the command span hooks to rootCmd (chaining any hooks already
// set), and runs the command inside a root span. Providers are flushed
// before the process exit code is returned: 0 on success, 1 on any
// initialization or command error (the error is logged before returning).
//
// An empty svc.Name defaults to rootCmd.Name(), then to the executable name.
//
// Do not assign the PersistentPreRunE/PersistentPostRunE hooks from this
// package on rootCmd when using Run: they are attached automatically, and
// attaching them twice starts duplicate spans.
func Run(ctx context.Context, svc Service, rootCmd *cobra.Command, opts ...ConfigOption) int {
	if rootCmd == nil {
		slog.Error("nil root command")
		return 1
	}
	svc = resolveServiceName(svc, rootCmd.Name())

	shutdown, err := initConfigAndTelemetry(ctx, svc, opts...)
	if err != nil {
		slog.With("error", err).Error("setup failed")
		return 1
	}

	instrumentCmd(rootCmd)
	cmdErr := CobraCmdExecute(ctx, rootCmd)
	shutdown() // flush buffered telemetry before the caller exits
	if cmdErr != nil {
		slog.With("error", cmdErr).Error("command failed")
		return 1
	}
	return 0
}

// instrumentCmd attaches the command span hooks to cmd, chaining any
// PersistentPreRun(E)/PersistentPostRun(E) hooks the command already has.
// The span hook runs first in pre-run (so the span exists for user hooks)
// and last in post-run, which always runs so the span ends even when a user
// hook fails.
func instrumentCmd(cmd *cobra.Command) {
	preE, pre := cmd.PersistentPreRunE, cmd.PersistentPreRun
	cmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		if err := PersistentPreRunE(c, args); err != nil {
			return err
		}
		if preE != nil {
			return preE(c, args)
		}
		if pre != nil {
			pre(c, args)
		}
		return nil
	}

	postE, post := cmd.PersistentPostRunE, cmd.PersistentPostRun
	cmd.PersistentPostRunE = func(c *cobra.Command, args []string) error {
		var err error
		if postE != nil {
			err = postE(c, args)
		} else if post != nil {
			post(c, args)
		}
		// Always run: it ends the span started by the pre-run hook.
		_ = PersistentPostRunE(c, args)
		return err
	}
}
