package setup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/eldius/cli-telemetry-helper/internal/telemetry"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Type aliases re-exporting the internal configuration types, so consumers
// only need to import this package.
type (
	// TelemetryOptions holds the service identity and the per-signal
	// backend configuration consumed by Setup.
	TelemetryOptions = telemetry.Options
	// Backends groups the traces, metrics and logs backend configurations.
	Backends = telemetry.Backends
	// Service identifies the instrumented application.
	Service = telemetry.Service
	// Backend configures a single OTLP/gRPC backend endpoint.
	Backend = telemetry.Backend
)

// Setup initializes the OTLP/gRPC tracer, meter and logger providers for
// every backend marked Enabled in cfg, registering them as the global OTel
// providers. If any backend fails to initialize, the ones already started
// are shut down before the error is returned.
func Setup(ctx context.Context, cfg TelemetryOptions) (ShutdownFunc, error) {
	backends := []struct {
		enabled bool
		setup   func(context.Context, telemetry.Options) (func(), error)
	}{
		{cfg.Backend.Traces.Enabled, telemetry.SetupTracer},
		{cfg.Backend.Metrics.Enabled, telemetry.SetupMeter},
		{cfg.Backend.Logs.Enabled, telemetry.SetupLogger},
	}

	var shutdownHooks []func()
	shutdownAll := func() {
		var wg sync.WaitGroup
		for _, hook := range shutdownHooks {
			wg.Go(hook)
		}
		wg.Wait()
	}

	for _, b := range backends {
		if !b.enabled {
			continue
		}
		shutdown, err := b.setup(ctx, cfg)
		if err != nil {
			shutdownAll() // clean up backends that were already initialized
			return telemetry.EmptyFunc, err
		}
		shutdownHooks = append(shutdownHooks, shutdown)
	}

	return shutdownAll, nil
}

// ShutdownFunc flushes pending telemetry and shuts down the initialized
// providers. Call it before the process exits or buffered data is lost.
type ShutdownFunc func()

// CobraCmdExecute runs rootCmd wrapped in a root span and returns the
// process exit code. It never calls os.Exit, so the caller can flush
// telemetry before terminating.
func CobraCmdExecute(ctx context.Context, rootCmd *cobra.Command) int {
	tracer := otel.GetTracerProvider().Tracer(telemetry.InstrumentationName)
	execName := filepath.Base(os.Args[0])
	pid := os.Getpid()
	spanCtx, span := tracer.Start(
		ctx,
		execName,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("process.executable.name", execName),
			attribute.Int("process.pid", pid),
			attribute.StringSlice("process.command_args", os.Args[1:]),
			attribute.String("process.executable.path", os.Args[0]),
			attribute.String("process.command", strings.Join(os.Args, " ")),
			attribute.String("otel.span.parent.origin", "none"),
		),
	)
	defer span.End()

	if err := rootCmd.ExecuteContext(spanCtx); err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attribute.Int("process.exit.code", 1))
		span.SetAttributes(attribute.String("error.type", "1"))
		return 1
	}

	return 0
}
