package health

import (
	"context"
	"net/http"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/response"
)

const readinessTimeout = 2 * time.Second

type DatabasePinger interface {
	Ping(context.Context) error
}

type Handler struct {
	environment    string
	databasePinger DatabasePinger
}

type healthResponse struct {
	Status      string `json:"status"`
	Environment string `json:"environment"`
}

func NewHealthCheckHandler(
	environment string,
	databasePinger DatabasePinger,
) *Handler {
	return &Handler{
		environment:    environment,
		databasePinger: databasePinger,
	}
}

func (handler *Handler) Live(
	w http.ResponseWriter,
	_ *http.Request,
) {
	handler.writeResponse(
		w,
		http.StatusOK,
		"ok",
	)
}

func (handler *Handler) Ready(
	w http.ResponseWriter,
	r *http.Request,
) {
	if handler.databasePinger == nil {
		handler.writeResponse(
			w,
			http.StatusServiceUnavailable,
			"unavailable",
		)

		return
	}

	ctx, cancel := context.WithTimeout(
		r.Context(),
		readinessTimeout,
	)
	defer cancel()

	if err := handler.databasePinger.Ping(ctx); err != nil {
		handler.writeResponse(
			w,
			http.StatusServiceUnavailable,
			"unavailable",
		)

		return
	}

	handler.writeResponse(
		w,
		http.StatusOK,
		"ok",
	)
}

func (handler *Handler) writeResponse(
	w http.ResponseWriter,
	statusCode int,
	status string,
) {
	data := healthResponse{
		Status:      status,
		Environment: handler.environment,
	}

	if err := response.WriteJSON(
		w,
		statusCode,
		data,
	); err != nil {
		http.Error(
			w,
			"the server encountered a problem",
			http.StatusInternalServerError,
		)
	}
}
