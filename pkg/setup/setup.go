// Package setup wires viper configuration and OpenTelemetry providers for
// cobra-based CLI applications.
package setup

import (
	"context"
	"fmt"
	"os"
	"reflect"

	"github.com/eldius/cli-telemetry-helper/internal/telemetry"
	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func expandEnvHook() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data any) (any, error) {
		if f.Kind() == reflect.String && t.Kind() == reflect.String {
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
func MainWrapper(ctx context.Context, svc Service, m func() error, opts ...ConfigOption) error {

	if err := InitViper(ctx, svc, opts...); err != nil {
		return err
	}

	var backends Backends
	if err := viper.UnmarshalKey("telemetry.backends", &backends, viper.DecodeHook(expandEnvHook())); err != nil {
		return err
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

	return m()
}

// PersistentPreRunE is a cobra hook that starts a span named after the
// command and stores it in the command context. Pair it with
// PersistentPostRunE on the same command to end the span.
func PersistentPreRunE(cmd *cobra.Command, args []string) error {
	return telemetry.PersistentPreRunE(cmd, args)
}
