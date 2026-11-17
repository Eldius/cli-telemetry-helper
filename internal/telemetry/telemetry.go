package telemetry

import (
	"time"

	"go.opentelemetry.io/otel/trace"
)

type tracingContextKey string

const tracingKey tracingContextKey = "tracing"

type tracingData struct {
	span  trace.Span
	start time.Time
}

type Options struct {
	Service Service  `json:"service" yaml:"service" mapstructure:"service"`
	Backend Backends `json:"backend" yaml:"backend" mapstructure:"backend"`
}

type Backends struct {
	Traces  Backend `json:"traces" yaml:"traces" mapstructure:"traces"`
	Metrics Backend `json:"metrics" yaml:"metrics" mapstructure:"metrics"`
	Logs    Backend `json:"logs" yaml:"logs" mapstructure:"logs"`
}

type Service struct {
	Name    string `json:"name" yaml:"name" mapstructure:"name"`
	Version string `json:"version" yaml:"version" mapstructure:"version"`
}

type Backend struct {
	Endpoint    string            `json:"endpoint" yaml:"endpoint" mapstructure:"endpoint"`
	AuthHeaders map[string]string `json:"auth_headers" yaml:"auth_headers" mapstructure:"auth_headers"`
	Enabled     bool              `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
}
