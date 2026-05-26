package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// accessLog emits one slog line per request after the handler returns. It must
// be registered AFTER middleware.RequestID + middleware.RealIP so the request
// id and real client IP are already populated.
//
// Health endpoints are skipped to keep the log readable; their failures show up
// in /readyz monitoring instead. The middleware also surfaces the chi request
// id as an X-Request-Id response header so clients can correlate a failed
// request with the server log line.
func accessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
				next.ServeHTTP(w, r)
				return
			}
			reqID := middleware.GetReqID(r.Context())
			if reqID != "" {
				w.Header().Set("X-Request-Id", reqID)
			}
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			defer func() {
				level := slog.LevelInfo
				switch {
				case ww.Status() >= 500:
					level = slog.LevelError
				case ww.Status() >= 400:
					level = slog.LevelWarn
				}
				logger.LogAttrs(r.Context(), level, "http_request",
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", ww.Status()),
					slog.Int("bytes", ww.BytesWritten()),
					slog.Duration("dur", time.Since(start)),
					slog.String("request_id", reqID),
					slog.String("remote", r.RemoteAddr),
				)
			}()
			next.ServeHTTP(ww, r)
		})
	}
}
