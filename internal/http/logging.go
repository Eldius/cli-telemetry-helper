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

		log.With("request", reqInfo).InfoContext(r.Context(), "IncomingHTTPRequestReceived")

		wWrapper := getResponseWriter(w)

		next.ServeHTTP(wWrapper, r)

		reqInfo.Response = wWrapper.Response()
		reqInfo.Duration = time.Since(start)

		log.With("request", reqInfo).InfoContext(r.Context(), "IncomingHTTPRequestResponse")
	})
}
