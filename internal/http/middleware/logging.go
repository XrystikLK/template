package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := newResponseWriter(w)

			next.ServeHTTP(rw, r)

			durationMs := time.Since(start).Milliseconds()

			routePattern := chi.RouteContext(r.Context()).RoutePattern()
			if routePattern == "" {
				routePattern = r.URL.Path
			}

			status := rw.Status()
			level := slog.LevelInfo
			if status >= 500 {
				level = slog.LevelError
			}

			if logger != nil {
				logger.Log(r.Context(), level, "request handled",
					slog.String("method", r.Method),
					slog.String("route", routePattern),
					slog.Int("status", status),
					slog.Int64("duration_ms", durationMs),
				)
			}
		})
	}
}

