// Package setup wires viper configuration and OpenTelemetry providers for
// cobra-based CLI applications.
package setup

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"reflect"

	"github.com/eldius/cli-telemetry-helper/internal/telemetry"
	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/cobra"
)

// expandEnvHook expands ${VAR} references in every string being decoded,
// regardless of the target type, so non-string fields (e.g. an int port set
// as "${PORT}") are expanded before weak type conversion.
func expandEnvHook() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, _ reflect.Type, data any) (any, error) {
		if f.Kind() == reflect.String {
			return os.ExpandEnv(data.(string)), nil
		}
		return data, nil
	}
}

// MainWrapper initializes configuration and telemetry around a CLI's main
// function: it runs InitViper, unmarshals "telemetry.backends" (expanding
// ${ENV_VAR} references in string values), sets up the enabled OTel
// providers, and invokes m. Providers are flushed and shut down after m
// returns; any initialization failure aborts before m runs.
func MainWrapper(ctx context.Context, svc Service, m func(ctx context.Context) error, opts ...ConfigOption) error {

	if err := InitViper(ctx, svc, opts...); err != nil {
		return err
	}

	var backends Backends
	if err := UnmarshalKey("telemetry.backends", &backends); err != nil {
		slog.With("error", err).Warn("failed to unmarshal telemetry backends")
	}

	cfg := TelemetryOptions{
		Service: svc,
		Backend: backends,
	}

	shutdown, err := Setup(ctx, cfg)
	if err != nil {
		return fmt.Errorf("setup telemetry: %w", err)
	}
	defer shutdown()

	return m(ctx)
}

// PersistentPreRunE is a cobra hook that starts a span named after the
// command and stores it in the command context. Pair it with
// PersistentPostRunE on the same command to end the span.
func PersistentPreRunE(cmd *cobra.Command, args []string) error {
	return telemetry.PersistentPreRunE(cmd, args)
}

// PersistentPostRunE is a cobra hook that marks the span started by
// PersistentPreRunE as successful and ends it.
func PersistentPostRunE(cmd *cobra.Command, args []string) error {
	return telemetry.PersistentPostRunE(cmd, args)
}
