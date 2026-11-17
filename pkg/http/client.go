// Package http provides an OpenTelemetry-instrumented HTTP client.
package http

import (
	"net/http"

	inthttp "github.com/eldius/cli-telemetry-helper/internal/http"
)

// NewHTTPClient returns a new instrumented HTTP client.
func NewHTTPClient() *http.Client {
	return inthttp.NewHTTPClient()
}
