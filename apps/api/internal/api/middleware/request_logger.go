package middleware

import (
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
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

			logger.Infow(
				"HTTP request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", responseWriter.Status(),
				"bytes", responseWriter.BytesWritten(),
				"duration_ms", time.Since(startedAt).Milliseconds(),
				"request_id", chimiddleware.GetReqID(r.Context()),
			)
		})
	}
}
