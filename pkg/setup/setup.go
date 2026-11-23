// Package setup wires viper configuration and OpenTelemetry providers for
// cobra-based CLI applications.
package setup

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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
// returns; any initialization failure aborts before m runs. An empty
// svc.Name defaults to the executable's base name.
func MainWrapper(ctx context.Context, svc Service, m func(ctx context.Context) error, opts ...ConfigOption) error {
	svc = resolveServiceName(svc, "")

	shutdown, err := initConfigAndTelemetry(ctx, svc, opts...)
	if err != nil {
		return err
	}
	defer shutdown()

	return m(ctx)
}

// resolveServiceName fills an empty service name with cmdName, falling back
// to the base name of the process executable.
func resolveServiceName(svc Service, cmdName string) Service {
	if svc.Name == "" {
		svc.Name = cmdName
	}
	if svc.Name == "" {
		svc.Name = filepath.Base(os.Args[0])
	}
	return svc
}

// initConfigAndTelemetry is the setup shared by MainWrapper and Run: it
// initializes viper, unmarshals "telemetry.backends" with ${VAR} expansion,
// and sets up the enabled OTel providers. The returned shutdown func flushes
// and shuts down the providers.
func initConfigAndTelemetry(ctx context.Context, svc Service, opts ...ConfigOption) (ShutdownFunc, error) {
	if err := InitViper(ctx, svc, opts...); err != nil {
		return nil, err
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
		return nil, fmt.Errorf("setup telemetry: %w", err)
	}
	return shutdown, nil
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
