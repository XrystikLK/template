package middleware

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

type ServerMetrics interface {
	RecordRequestDuration(ctx context.Context, durationSec float64, method, route, statusCode string)
	AddActiveRequests(ctx context.Context, delta int64, method, route string)
}

func Metrics(m ServerMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if m == nil {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			routePattern := chi.RouteContext(r.Context()).RoutePattern()
			if routePattern == "" {
				routePattern = r.URL.Path
			}

			m.AddActiveRequests(r.Context(), 1, r.Method, routePattern)
			defer m.AddActiveRequests(r.Context(), -1, r.Method, routePattern)

			rw := newResponseWriter(w)
			next.ServeHTTP(rw, r)

			finalRoute := chi.RouteContext(r.Context()).RoutePattern()
			if finalRoute == "" {
				finalRoute = routePattern
			}

			durationSec := time.Since(start).Seconds()
			m.RecordRequestDuration(r.Context(), durationSec, r.Method, finalRoute, strconv.Itoa(rw.Status()))
		})
	}
}
