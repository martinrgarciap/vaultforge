package health

import (
	"net/http"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/response"
)

type Handler struct {
	environment string
}

type healthResponse struct {
	Status      string `json:"status"`
	Environment string `json:"environment"`
}

func NewHealthCheckHandler(environment string) *Handler {
	return &Handler{
		environment: environment,
	}
}

func (handler *Handler) HealthCheck(
	w http.ResponseWriter,
	r *http.Request,
) {
	data := healthResponse{
		Status:      "ok",
		Environment: handler.environment,
	}

	if err := response.WriteJSON(
		w,
		http.StatusOK,
		data,
	); err != nil {
		http.Error(
			w,
			"the server encountered a problem",
			http.StatusInternalServerError,
		)
	}
}
