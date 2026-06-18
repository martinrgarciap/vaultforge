package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	appmiddleware "github.com/martinrgarciap/vaultforge/apps/api/internal/api/middleware"
)

func (app *Application) Routes() http.Handler {
	router := chi.NewRouter()

	router.Use(chimiddleware.RequestID)
	router.Use(appmiddleware.RequestLogger(app.logger))
	router.Use(appmiddleware.RecoverPanic(app.logger))
	router.Use(appmiddleware.SecurityHeaders)
	router.Use(chimiddleware.Timeout(60 * time.Second))

	router.NotFound(app.notFoundResponse)
	router.MethodNotAllowed(app.methodNotAllowedResponse)

	router.Get("/health", app.healthHandler.HealthCheck)

	return router
}
