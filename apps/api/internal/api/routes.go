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

		router.Group(func(router chi.Router) {
			router.Use(appmiddleware.RequireAuthentication(app.sessionService, app.logger))

			router.Route("/sessions", func(router chi.Router) {
				router.Get("/", app.sessionHandler.List)
				router.Delete("/", app.sessionHandler.LogoutAll)
				router.Delete("/current", app.sessionHandler.LogoutCurrent)
				router.Delete("/{sessionID}", app.sessionHandler.Revoke)
			})

			router.Route("/vaults", func(router chi.Router) {
				router.Post("/", app.vaultHandler.Create)
				router.Get("/", app.vaultHandler.List)

				router.Route("/{vaultID}", func(router chi.Router) {
					router.Get("/", app.vaultHandler.Get)
					router.Patch("/", app.vaultHandler.Rename)
					router.Delete("/", app.vaultHandler.Delete)

					router.Route("/items", func(router chi.Router) {
						router.Post("/", app.itemHandler.Create)
						router.Get("/", app.itemHandler.List)

						router.Route("/{itemID}", func(router chi.Router) {
							router.Get("/", app.itemHandler.Get)
							router.Put("/", app.itemHandler.Update)
							router.Delete("/", app.itemHandler.SoftDelete)
							router.Post("/restore", app.itemHandler.Restore)
							router.Delete("/permanent", app.itemHandler.PermanentDelete)
						})
					})
				})
			})
		})
	})

	return router
}
