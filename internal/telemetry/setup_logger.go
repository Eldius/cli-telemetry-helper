package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/sdk/log"
	"google.golang.org/grpc/encoding/gzip"
)

var (
	ErrLoggerInitialization     = errors.New("logger initialization")
	ErrLoggerConnectionShutdown = errors.New("logger connection shutdown")
)

func SetupLogger(ctx context.Context, cfg Options) (func(), error) {
	exporter, err := newLoggetExporter(ctx, cfg)
	if err != nil {
		return EmptyFunc, fmt.Errorf("%w:%w", ErrLoggerInitialization, err)
	}
	provider := newLoggerProvider(ctx, exporter, cfg)
	otel.SetLoggerProvider(provider)

	slog.SetDefault(otelslog.NewLogger(
		"github.com/eldius/cli-telemetry-helper",
		otelslog.WithLoggerProvider(provider),
		otelslog.WithSource(true),
		otelslog.WithVersion(cfg.Service.Version),
	))

	//logFile, err := os.OpenFile("execution.log", os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0755)
	//slog.SetDefault(slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{
	//	AddSource:   true,
	//	Level:       slog.LevelDebug,
	//	ReplaceAttr: nil,
	//})))

	return func() {
		slog.Debug("Logger provider shutting down")
		if err := exporter.ForceFlush(ctx); err != nil {
			err = fmt.Errorf("%w:%w", ErrLoggerConnectionShutdown, err)
			slog.With("error", err).Warn("log exporter flush failed")
		}
		if err := provider.ForceFlush(ctx); err != nil {
			err = fmt.Errorf("%w:%w", ErrLoggerConnectionShutdown, err)
			slog.With("error", err).Warn("logger provider flush failed")
		}
		if err := exporter.Shutdown(ctx); err != nil {
			err = fmt.Errorf("%w:%w", ErrLoggerConnectionShutdown, err)
			slog.With("error", err).Warn("log exporter shutdown failed")
		}
		if err := provider.Shutdown(ctx); err != nil {
			err = fmt.Errorf("%w:%w", ErrLoggerConnectionShutdown, err)
			slog.With("error", err).Warn("logger provider shutdown failed")
		}
	}, nil
}

func newLoggetExporter(ctx context.Context, cfg Options) (*otlploggrpc.Exporter, error) {
	exporter, err := otlploggrpc.New(ctx,
		otlploggrpc.WithInsecure(), // use http & not https
		otlploggrpc.WithEndpoint(cfg.Backend.Logs.Endpoint),
		otlploggrpc.WithCompressor(gzip.Name),
		otlploggrpc.WithTimeout(10*time.Second),
		otlploggrpc.WithHeaders(cfg.Backend.Logs.AuthHeaders),
	)
	if err != nil {
		return nil, fmt.Errorf("%w:%w", ErrLoggerInitialization, err)
	}

	return exporter, nil
}

func newLoggerProvider(_ context.Context, exp log.Exporter, cfg Options) *log.LoggerProvider {
	res := getDefaultResources(cfg)

	processor := log.NewBatchProcessor(
		exp,
	)
	lp := log.NewLoggerProvider(
		log.WithResource(res),
		log.WithProcessor(processor),
	)

	return lp
}
