package http

import (
	"log/slog"
	"net/http"
	"time"
)

// LoggingMiddleware is a middleware that logs HTTP requests and responses.
// It extracts the request body and logs it. It also logs the request and response details.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := slog.With(
			"pkg", "http_server_logging",
		)
		start := time.Now()
		reqInfo := HTTPRequestLogRecord{
			Pattern: r.Pattern,
			URL:     r.URL.String(),
			Method:  r.Method,
			Request: HTTPRequestData{
				Headers: r.Header,
			},
		}
		reqInfo.Request.Body = ExtractRequestBody(r)

		log = log.With(
			"http.url", reqInfo.URL,
			"http.url_pattern", reqInfo.Pattern,
			"http.method", reqInfo.Method,
			"http.request.headers", redactHeaders(reqInfo.Request.Headers),
			"http.request.body", truncateBody(reqInfo.Request.Body),
		)
		log.DebugContext(ctx, "IncomingHTTPRequestReceived")

		wWrapper := getResponseWriter(w)

		next.ServeHTTP(wWrapper, r)

		reqInfo.Response = wWrapper.Response()
		reqInfo.Duration = time.Since(start)

		log.With(
			"http.response.headers", redactHeaders(reqInfo.Response.Headers),
			"http.response.body", truncateBody(reqInfo.Response.Body),
			"http.response.status_code", reqInfo.Response.StatusCode,
			"http.duration", reqInfo.Duration,
		).DebugContext(ctx, "IncomingHTTPRequestResponse")

	})
}
