package api

import (
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/health"

	"go.uber.org/zap"
)

type Application struct {
	config        Config
	logger        *zap.SugaredLogger
	healthHandler *health.Handler
}

func NewApplication(
	cfg Config,
	logger *zap.SugaredLogger,
	databasePinger health.DatabasePinger,
) *Application {
	return &Application{
		config: cfg,
		logger: logger,
		healthHandler: health.NewHealthCheckHandler(
			cfg.Env,
			databasePinger,
		),
	}
}
