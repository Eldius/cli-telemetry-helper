package telemetry

import (
	"time"

	"go.opentelemetry.io/otel/trace"
)

type tracingContextKey string

const tracingKey tracingContextKey = "tracing"

// InstrumentationName is the OpenTelemetry instrumentation scope name used
// by all tracers, meters and loggers created by this module.
const InstrumentationName = "github.com/eldius/cli-telemetry-helper"

type tracingData struct {
	span  trace.Span
	start time.Time
}

// Options holds the full configuration for initializing the OTel providers:
// the service identity plus one Backend per telemetry signal.
type Options struct {
	Service Service  `json:"service" yaml:"service" mapstructure:"service"`
	Backend Backends `json:"backend" yaml:"backend" mapstructure:"backend"`
}

// Backends groups the per-signal backend configurations.
type Backends struct {
	Traces  Backend `json:"traces" yaml:"traces" mapstructure:"traces"`
	Metrics Backend `json:"metrics" yaml:"metrics" mapstructure:"metrics"`
	Logs    Backend `json:"logs" yaml:"logs" mapstructure:"logs"`
}

// Service identifies the instrumented application; Name and Version become
// the OTel resource's service.name and service.version.
type Service struct {
	Name    string `json:"name" yaml:"name" mapstructure:"name"`
	Version string `json:"version" yaml:"version" mapstructure:"version"`
}

// Backend configures a single OTLP/gRPC endpoint. Enabled gates whether the
// backend is initialized; AuthHeaders are sent as gRPC metadata.
type Backend struct {
	Endpoint    string            `json:"endpoint" yaml:"endpoint" mapstructure:"endpoint"`
	AuthHeaders map[string]string `json:"auth_headers" yaml:"auth_headers" mapstructure:"auth_headers"`
	Enabled     bool              `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
}
