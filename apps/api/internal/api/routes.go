package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	appmiddleware "github.com/martinrgarciap/vaultforge/apps/api/internal/api/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/buildinfo"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/ratelimit"
)

func (app *Application) Routes() http.Handler {
	router := chi.NewRouter()

	router.Use(appmiddleware.BoundedRequestID)
	router.Use(appmiddleware.TraceRequests(buildinfo.ServiceName))
	router.Use(appmiddleware.RequestLogger(app.logger))
	router.Use(app.metricsRegistry.Middleware)
	router.Use(appmiddleware.RecoverPanic(app.logger))
	router.Use(appmiddleware.SecurityHeaders)
	router.Use(
		appmiddleware.RequestTimeout(
			app.config.HTTP.RequestTimeout(),
		),
	)

	router.NotFound(app.notFoundResponse)
	router.MethodNotAllowed(app.methodNotAllowedResponse)

	router.Route("/health", func(router chi.Router) {
		router.Get("/", app.healthHandler.Live)
		router.Get("/live", app.healthHandler.Live)
		router.Get("/ready", app.healthHandler.Ready)
		router.Get("/diagnostics", app.diagnosticsHandler.Get)
	})

	router.With(appmiddleware.RequireLoopback).Get(
		"/internal/metrics",
		app.metricsRegistry.ServeHTTP,
	)

	router.Route("/v1", func(router chi.Router) {
		router.Route("/auth", func(router chi.Router) {
			router.With(
				app.authenticationRateLimit(
					ratelimit.ScopeRegistration,
					app.config.RateLimits.Registration,
				),
			).Post("/register", app.authHandler.Register)

			router.With(
				app.authenticationRateLimit(
					ratelimit.ScopeLogin,
					app.config.RateLimits.Login,
				),
			).Post("/login", app.authHandler.Login)

			router.With(
				app.authenticationRateLimit(
					ratelimit.ScopeRefresh,
					app.config.RateLimits.Refresh,
				),
			).Post("/refresh", app.authHandler.Refresh)
		})

		router.Group(func(router chi.Router) {
			router.Use(
				appmiddleware.RequireAuthentication(
					app.sessionService,
					app.logger,
				),
			)

			router.Route("/sessions", func(router chi.Router) {
				router.Get("/", app.sessionHandler.List)
				router.Delete("/", app.sessionHandler.LogoutAll)
				router.Delete("/current", app.sessionHandler.LogoutCurrent)
				router.Delete("/{sessionID}", app.sessionHandler.Revoke)
			})

			router.Route("/vaults", func(router chi.Router) {
				router.With(
					app.mutationRateLimit(),
				).Post("/", app.vaultHandler.Create)

				router.Get("/", app.vaultHandler.List)

				router.Route("/{vaultID}", func(router chi.Router) {
					router.Get("/", app.vaultHandler.Get)

					router.With(
						app.mutationRateLimit(),
					).Put("/crypto", app.vaultHandler.InitializeCrypto)

					router.With(
						app.mutationRateLimit(),
					).Patch("/", app.vaultHandler.Rename)

					router.With(
						app.mutationRateLimit(),
					).Delete("/", app.vaultHandler.Delete)

					router.Route("/items", func(router chi.Router) {
						router.With(
							app.mutationRateLimit(),
						).Post("/", app.itemHandler.Create)

						router.Get("/", app.itemHandler.List)

						router.Route("/{itemID}", func(router chi.Router) {
							router.Get("/", app.itemHandler.Get)

							router.With(
								app.mutationRateLimit(),
							).Put("/", app.itemHandler.Update)

							router.With(
								app.mutationRateLimit(),
							).Delete("/", app.itemHandler.SoftDelete)

							router.With(
								app.mutationRateLimit(),
							).Post("/restore", app.itemHandler.Restore)

							router.With(
								app.mutationRateLimit(),
							).Delete(
								"/permanent",
								app.itemHandler.PermanentDelete,
							)
						})
					})
				})
			})
		})
	})

	return router
}

func (app *Application) authenticationRateLimit(
	scope string,
	policy ratelimit.Policy,
) func(http.Handler) http.Handler {
	return appmiddleware.RateLimitByPeerIP(
		app.securityEnforcer,
		scope,
		policy,
		"authentication_unavailable",
		"Authentication is temporarily unavailable.",
		app.logger,
	)
}

func (app *Application) mutationRateLimit() func(
	http.Handler,
) http.Handler {
	return appmiddleware.RateLimitByPrincipal(
		app.securityEnforcer,
		ratelimit.ScopeMutation,
		app.config.RateLimits.Mutation,
		"service_unavailable",
		"The service is temporarily unavailable.",
		app.logger,
	)
}
