package middleware

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const unmatchedRoute = "unmatched"

func TraceRequests(serviceName string) func(http.Handler) http.Handler {
	return traceRequests(
		serviceName,
		otel.GetTracerProvider(),
		otel.GetTextMapPropagator(),
	)
}

func traceRequests(
	serviceName string,
	provider trace.TracerProvider,
	propagator propagation.TextMapPropagator,
) func(http.Handler) http.Handler {
	tracer := provider.Tracer(serviceName + "/http")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			parentContext := propagator.Extract(
				r.Context(),
				propagation.HeaderCarrier(r.Header),
			)

			traceContext, span := tracer.Start(
				parentContext,
				"HTTP "+r.Method,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(attribute.String("http.request.method", r.Method)),
			)
			defer span.End()

			responseWriter := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

			tracedRequest := r.WithContext(traceContext)
			next.ServeHTTP(responseWriter, tracedRequest)

			route := normalizedRoute(tracedRequest)
			status := responseWriter.Status()
			if status == 0 {
				status = http.StatusOK
			}

			span.SetName(r.Method + " " + route)

			span.SetAttributes(
				attribute.String("http.route", route),
				attribute.Int("http.response.status_code", status),
			)

			if status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, http.StatusText(status))
			}
		})
	}
}

func normalizedRoute(r *http.Request) string {
	routeContext := chi.RouteContext(r.Context())
	if routeContext == nil {
		return unmatchedRoute
	}

	route := strings.TrimSpace(routeContext.RoutePattern())
	if route == "" {
		return unmatchedRoute
	}

	if route != "/" {
		route = strings.TrimSuffix(route, "/")
	}

	if route == "" {
		return unmatchedRoute
	}

	return route
}
