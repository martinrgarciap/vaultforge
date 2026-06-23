package diagnostics

import (
	"net/http"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/response"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/buildinfo"
)

type Handler struct {
	info buildinfo.Info
}

func New(info buildinfo.Info) *Handler {
	return &Handler{
		info: buildinfo.New(info.Version, info.Commit),
	}
}

func (handler *Handler) Get(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	if err := response.WriteJSON(w, http.StatusOK, handler.info); err != nil {
		http.Error(w, "The server encountered a problem.", http.StatusInternalServerError)
	}
}
