package api

import (
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/response"
)

func (app *Application) internalServerResponse(
	w http.ResponseWriter,
	r *http.Request,
	err error,
) {
	requestID := middleware.GetReqID(r.Context())

	app.logger.Errorw(
		"internal server error",
		"method", r.Method,
		"path", r.URL.Path,
		"request_id", requestID,
		"error", err,
	)

	if writeErr := response.WriteError(
		w,
		http.StatusInternalServerError,
		"internal_server_error",
		"The server encountered a problem.",
		requestID,
	); writeErr != nil {
		app.logger.Errorw(
			"failed to write error response",
			"request_id", requestID,
			"error", writeErr,
		)
	}
}

func (app *Application) notFoundResponse(
	w http.ResponseWriter,
	r *http.Request,
) {
	requestID := middleware.GetReqID(r.Context())

	app.logger.Warnw(
		"route not found",
		"method", r.Method,
		"path", r.URL.Path,
		"request_id", requestID,
	)

	if err := response.WriteError(
		w,
		http.StatusNotFound,
		"not_found",
		"The requested resource was not found.",
		requestID,
	); err != nil {
		app.logger.Errorw(
			"failed to write error response",
			"request_id", requestID,
			"error", err,
		)
	}
}

func (app *Application) methodNotAllowedResponse(
	w http.ResponseWriter,
	r *http.Request,
) {
	requestID := middleware.GetReqID(r.Context())

	app.logger.Warnw(
		"method not allowed",
		"method", r.Method,
		"path", r.URL.Path,
		"request_id", requestID,
	)

	if err := response.WriteError(
		w,
		http.StatusMethodNotAllowed,
		"method_not_allowed",
		"The requested method is not allowed.",
		requestID,
	); err != nil {
		app.logger.Errorw(
			"failed to write error response",
			"request_id", requestID,
			"error", err,
		)
	}
}
