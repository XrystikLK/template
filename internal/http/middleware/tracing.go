package middleware

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

func Tracing(tracer trace.Tracer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			initialName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
			ctx, span := tracer.Start(ctx, initialName,
				trace.WithSpanKind(trace.SpanKindServer),
			)
			defer span.End()

			r = r.WithContext(ctx)
			rw := newResponseWriter(w)

			next.ServeHTTP(rw, r)

			routePattern := chi.RouteContext(r.Context()).RoutePattern()
			if routePattern == "" {
				routePattern = r.URL.Path
			}

			span.SetName(fmt.Sprintf("%s %s", r.Method, routePattern))
			span.SetAttributes(
				semconv.HTTPRequestMethodKey.String(r.Method),
				semconv.HTTPResponseStatusCodeKey.Int(rw.Status()),
				attribute.String("http.route", routePattern),
			)

			if rw.Status() >= 500 {
				span.SetStatus(codes.Error, http.StatusText(rw.Status()))
				span.SetAttributes(attribute.String("error.type", "internal_error"))
			}
		})
	}
}
