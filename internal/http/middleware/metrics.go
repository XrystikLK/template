package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/XrystikLK/template/internal/infra/telemetry"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func Metrics(m *telemetry.Metrics) func(http.Handler) http.Handler {
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

			activeAttrs := metric.WithAttributes(
				attribute.String("http.request.method", r.Method),
				attribute.String("http.route", routePattern),
			)

			m.ActiveRequests.Add(r.Context(), 1, activeAttrs)
			defer m.ActiveRequests.Add(r.Context(), -1, activeAttrs)

			rw := newResponseWriter(w)
			next.ServeHTTP(rw, r)

			finalRoute := chi.RouteContext(r.Context()).RoutePattern()
			if finalRoute == "" {
				finalRoute = routePattern
			}

			durationSec := time.Since(start).Seconds()
			durationAttrs := metric.WithAttributes(
				attribute.String("http.request.method", r.Method),
				attribute.String("http.route", finalRoute),
				attribute.String("http.response.status_code", strconv.Itoa(rw.Status())),
			)

			m.RequestDuration.Record(r.Context(), durationSec, durationAttrs)
		})
	}
}

