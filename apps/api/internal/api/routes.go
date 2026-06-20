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

	router.Route("/health", func(router chi.Router) {
		router.Get("/", app.healthHandler.Live)
		router.Get("/live", app.healthHandler.Live)
		router.Get("/ready", app.healthHandler.Ready)
	})

	router.Route("/v1", func(router chi.Router) {
		router.Route("/auth", func(router chi.Router) {
			router.Post("/register", app.authHandler.Register)
			router.Post("/login", app.authHandler.Login)
			router.Post("/refresh", app.authHandler.Refresh)
		})
	})

	return router
}
