package api

import (
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/authhandler"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/health"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"

	"go.uber.org/zap"
)

type Application struct {
	config         Config
	logger         *zap.SugaredLogger
	healthHandler  *health.Handler
	authHandler    *authhandler.Handler
	sessionService *session.Service
}

func NewApplication(
	cfg Config,
	logger *zap.SugaredLogger,
	databasePinger health.DatabasePinger,
	authService authhandler.RegistrationService,
	sessionService *session.Service,
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
			sessionService,
			logger,
		),
		sessionService: sessionService,
	}
}
