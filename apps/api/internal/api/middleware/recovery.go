package middleware

import (
	"fmt"
	"net/http"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/response"
	"go.uber.org/zap"
)

func RecoverPanic(
	logger *zap.SugaredLogger,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}

				requestID := chimiddleware.GetReqID(r.Context())

				logger.Errorw(
					"panic recovered",
					"method", r.Method,
					"path", r.URL.Path,
					"request_id", requestID,
					"panic_type", fmt.Sprintf("%T", recovered),
				)

				if err := response.WriteError(
					w,
					http.StatusInternalServerError,
					"internal_server_error",
					"The server encountered a problem.",
					requestID,
				); err != nil {
					logger.Errorw(
						"failed to write panic response",
						"request_id", requestID,
						"error", err,
					)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
