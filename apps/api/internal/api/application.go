package api

import (
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/authhandler"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/health"

	"go.uber.org/zap"
)

type Application struct {
	config        Config
	logger        *zap.SugaredLogger
	healthHandler *health.Handler
	authHandler   *authhandler.Handler
}

func NewApplication(
	cfg Config,
	logger *zap.SugaredLogger,
	databasePinger health.DatabasePinger,
	authService authhandler.Service,
) *Application {
	return &Application{
		config: cfg,
		logger: logger,
		healthHandler: health.NewHealthCheckHandler(
			cfg.Env,
			databasePinger,
		),
		authHandler: authhandler.New(
			authService,
			logger,
		),
	}
}
