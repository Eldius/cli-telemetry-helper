package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc/encoding/gzip"
)

const (
	exporterTimeout       = 10 * time.Second
	metricCollectInterval = 3 * time.Second
)

// EmptyFunc is a no-op shutdown function returned when setup fails.
func EmptyFunc() {}

type shutdowner interface {
	Shutdown(context.Context) error
}

// setupBackend creates an OTLP/gRPC exporter and SDK provider for one
// telemetry signal, registers the provider globally, and returns a shutdown
// function that flushes pending telemetry and closes the whole pipeline.
func setupBackend[P shutdowner, E any](
	ctx context.Context,
	name string,
	backend Backend,
	res *resource.Resource,
	newExporter func(context.Context, Backend) (E, error),
	newProvider func(*resource.Resource, E) P,
	register func(P),
) (func(), error) {
	exporter, err := newExporter(ctx, backend)
	if err != nil {
		return EmptyFunc, fmt.Errorf("%s exporter initialization: %w", name, err)
	}

	p := newProvider(res, exporter)
	register(p)

	return func() {
		slog.Debug(name + " provider shutting down")
		if err := p.Shutdown(ctx); err != nil {
			slog.With("error", err).Warn(name + " provider shutdown failed")
		}
	}, nil
}

// SetupTracer creates the OTLP/gRPC span exporter and TracerProvider, sets
// the provider as global (with TraceContext+Baggage propagation), and returns
// a function that flushes and shuts down the pipeline.
func SetupTracer(ctx context.Context, cfg Options) (func(), error) {
	return setupBackend(ctx, "traces", cfg.Backend.Traces, getDefaultResources(cfg),
		func(ctx context.Context, b Backend) (*otlptrace.Exporter, error) {
			return otlptracegrpc.New(ctx,
				otlptracegrpc.WithInsecure(), // use http & not https
				otlptracegrpc.WithEndpoint(b.Endpoint),
				otlptracegrpc.WithCompressor(gzip.Name),
				otlptracegrpc.WithTimeout(exporterTimeout),
				otlptracegrpc.WithHeaders(b.AuthHeaders),
			)
		},
		func(res *resource.Resource, exp *otlptrace.Exporter) *sdktrace.TracerProvider {
			return sdktrace.NewTracerProvider(
				sdktrace.WithSampler(sdktrace.AlwaysSample()),
				sdktrace.WithResource(res),
				sdktrace.WithBatcher(exp),
			)
		},
		func(tp *sdktrace.TracerProvider) {
			otel.SetTracerProvider(tp)
			otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
				propagation.TraceContext{},
				propagation.Baggage{},
			))
		},
	)
}

// SetupMeter creates the OTLP/gRPC metrics exporter and MeterProvider, sets
// the provider as global, and starts Go runtime metric collection. The
// returned function flushes and shuts down the pipeline.
func SetupMeter(ctx context.Context, cfg Options) (func(), error) {
	shutdown, err := setupBackend(ctx, "metrics", cfg.Backend.Metrics, getDefaultResources(cfg),
		func(ctx context.Context, b Backend) (*otlpmetricgrpc.Exporter, error) {
			return otlpmetricgrpc.New(ctx,
				otlpmetricgrpc.WithInsecure(), // use http & not https
				otlpmetricgrpc.WithEndpoint(b.Endpoint),
				otlpmetricgrpc.WithCompressor(gzip.Name),
				otlpmetricgrpc.WithTimeout(exporterTimeout),
				otlpmetricgrpc.WithHeaders(b.AuthHeaders),
			)
		},
		func(res *resource.Resource, exp *otlpmetricgrpc.Exporter) *metric.MeterProvider {
			return metric.NewMeterProvider(
				metric.WithResource(res),
				metric.WithReader(metric.NewPeriodicReader(exp,
					metric.WithInterval(metricCollectInterval))),
			)
		},
		func(mp *metric.MeterProvider) {
			otel.SetMeterProvider(mp)
		},
	)
	if err != nil {
		return EmptyFunc, err
	}
	// Start go runtime metric collection.
	if err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(metricCollectInterval)); err != nil {
		shutdown()
		return EmptyFunc, fmt.Errorf("metrics runtime instrumentation: %w", err)
	}
	return shutdown, nil
}

// SetupLogger creates the OTLP/gRPC log exporter and LoggerProvider, sets
// the provider as global, and replaces slog's default logger with an
// OTel-bridged one. The returned function flushes and shuts down the pipeline.
func SetupLogger(ctx context.Context, cfg Options) (func(), error) {
	return setupBackend(ctx, "logs", cfg.Backend.Logs, getDefaultResources(cfg),
		func(ctx context.Context, b Backend) (*otlploggrpc.Exporter, error) {
			return otlploggrpc.New(ctx,
				otlploggrpc.WithInsecure(), // use http & not https
				otlploggrpc.WithEndpoint(b.Endpoint),
				otlploggrpc.WithCompressor(gzip.Name),
				otlploggrpc.WithTimeout(exporterTimeout),
				otlploggrpc.WithHeaders(b.AuthHeaders),
			)
		},
		func(res *resource.Resource, exp *otlploggrpc.Exporter) *log.LoggerProvider {
			return log.NewLoggerProvider(
				log.WithResource(res),
				log.WithProcessor(log.NewBatchProcessor(exp)),
			)
		},
		func(lp *log.LoggerProvider) {
			otel.SetLoggerProvider(lp)
			slog.SetDefault(otelslog.NewLogger(
				InstrumentationName,
				otelslog.WithLoggerProvider(lp),
				otelslog.WithSource(true),
				otelslog.WithVersion(cfg.Service.Version),
			))
		},
	)
}
