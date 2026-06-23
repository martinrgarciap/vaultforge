package middleware

import (
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func RequestLogger(
	logger *zap.SugaredLogger,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			startedAt := time.Now()

			responseWriter := chimiddleware.NewWrapResponseWriter(
				w,
				r.ProtoMajor,
			)

			next.ServeHTTP(responseWriter, r)

			fields := []any{
				"method", r.Method,
				"route", normalizedRoute(r),
				"status", responseWriter.Status(),
				"bytes", responseWriter.BytesWritten(),
				"duration_ms", time.Since(startedAt).Milliseconds(),
				"request_id", chimiddleware.GetReqID(r.Context()),
			}

			spanContext := trace.SpanContextFromContext(r.Context())
			if spanContext.IsValid() {
				fields = append(fields, "trace_id", spanContext.TraceID().String())
			}

			logger.Infow(
				"HTTP request completed",
				fields...,
			)
		})
	}
}
