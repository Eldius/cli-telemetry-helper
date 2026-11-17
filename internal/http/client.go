package http

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func NewHTTPClient() *http.Client {
	return &http.Client{
		Transport: &loggingRoundTripper{
			proxied: otelhttp.NewTransport(http.DefaultTransport),
		},
	}
}

// loggingRoundTripper is a struct that implements the http.RoundTripper interface.
// It wraps an existing http.RoundTripper to add logging functionality.
type loggingRoundTripper struct {
	proxied http.RoundTripper
}

// RoundTrip is the core of the interceptor. It's called for each HTTP request.
// It logs request and response details and measures the request duration.
func (lrt *loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	log := slog.With("pkg", "http_client_logging")
	logData := HTTPRequestLogRecord{
		URL:    req.URL.String(),
		Method: req.Method,
		Request: HTTPRequestData{
			Headers: req.Header,
			Body:    ExtractRequestBody(req),
		},
	}
	//log.With("request", logData).DebugContext(ctx, "HTTPRemoteRequestStarting")
	log.With(
		"http.url", logData.URL,
		"http.method", logData.Method,
		"http.request.headers", redactHeaders(logData.Request.Headers),
		"http.request.body", truncateBody(logData.Request.Body),
	).DebugContext(ctx, "HTTPRemoteRequestStarting")

	start := time.Now()

	resp, err := lrt.proxied.RoundTrip(req)

	if err != nil {
		log.With("error", err, "request", logData).ErrorContext(ctx, "HTTPRequestFailed")
		return nil, err
	}
	logData.Duration = time.Since(start)
	if resp != nil {
		logData.Response = HTTPResponseData{
			Headers:    resp.Header,
			Body:       extractResponseBody(resp),
			StatusCode: resp.StatusCode,
		}
	}

	//log.With("request", logData).DebugContext(ctx, "HTTPRemoteRequestFinished")
	log.With(
		"http.url", logData.URL,
		"http.method", logData.Method,
		"http.request.headers", redactHeaders(logData.Request.Headers),
		"http.request.body", truncateBody(logData.Request.Body),
		"http.response.headers", redactHeaders(logData.Response.Headers),
		"http.response.body", truncateBody(logData.Response.Body),
		"http.response.status_code", logData.Response.StatusCode,
		"http.duration", logData.Duration,
	).DebugContext(ctx, "HTTPRemoteRequestFinished")

	return resp, nil
}

const maxLoggedBodyBytes = 4 * 1024

// truncateBody caps the logged body size to avoid flooding the log backend.
func truncateBody(body string) string {
	if len(body) <= maxLoggedBodyBytes {
		return body
	}
	return body[:maxLoggedBodyBytes] + "...[truncated]"
}

// redactHeaders masks credentials and session headers before logging.
func redactHeaders(h map[string][]string) map[string][]string {
	redacted := make(map[string][]string, len(h))
	for k, v := range h {
		switch http.CanonicalHeaderKey(k) {
		case "Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie", "X-Api-Key":
			redacted[k] = []string{"[REDACTED]"}
		default:
			redacted[k] = v
		}
	}
	return redacted
}

func extractResponseBody(res *http.Response) string {
	if res == nil {
		slog.With("event", "extractResponseBody").Debug("NullResponse")
		return ""
	}
	if res.Body == nil {
		return ""
	}
	reader := res.Body
	defer func() {
		_ = reader.Close()
	}()
	body, _ := io.ReadAll(reader)
	res.Body = io.NopCloser(bytes.NewBuffer(body))
	return string(body)
}

func ExtractRequestBody(req *http.Request) string {
	if req.Body == nil {
		return ""
	}
	reader := req.Body
	defer func() {
		_ = reader.Close()
	}()
	body, _ := io.ReadAll(reader)
	req.Body = io.NopCloser(bytes.NewBuffer(body))
	return string(body)
}

type HTTPRequestLogRecord struct {
	Request  HTTPRequestData  `json:"request"`
	Response HTTPResponseData `json:"response"`
	Duration time.Duration    `json:"duration,omitempty"`
	Pattern  string           `json:"pattern,omitempty"`
	URL      string           `json:"url,omitempty"`
	Method   string           `json:"method,omitempty"`
}

type HTTPRequestData struct {
	Body    string              `json:"body,omitempty"`
	Headers map[string][]string `json:"headers,omitempty"`
}

type HTTPResponseData struct {
	Headers    map[string][]string `json:"headers,omitempty"`
	Body       string              `json:"body,omitempty"`
	StatusCode int                 `json:"status_code,omitempty"`
}
