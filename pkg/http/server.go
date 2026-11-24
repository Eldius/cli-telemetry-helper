package http

import "github.com/eldius/cli-telemetry-helper/internal/http"

// ServerMuxLoggingMiddleware is a middleware that logs HTTP requests and responses.
// It extracts the request body and logs it. It also logs the request and response details.
var ServerMuxLoggingMiddleware = http.LoggingMiddleware

// ServerMuxTelemetryMiddleware is a middleware that wraps the provided http.ServeMux with OpenTelemetry instrumentation.
// It instruments the ServeMux with the otelhttp.NewHandler function and enables logging for all requests handled by the ServeMux.
var ServerMuxTelemetryMiddleware = http.TelemetryMiddleware
