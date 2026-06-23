package api

import (
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/authhandler"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/health"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/itemhandler"
	appmiddleware "github.com/martinrgarciap/vaultforge/apps/api/internal/api/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/sessioncookie"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/sessionhandler"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/vaulthandler"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"

	"go.uber.org/zap"
)

type SecurityEnforcer interface {
	appmiddleware.RequestLimiter
	authhandler.LoginProtector
}

type Application struct {
	config           Config
	logger           *zap.SugaredLogger
	healthHandler    *health.Handler
	securityEnforcer SecurityEnforcer
	authHandler      *authhandler.Handler
	sessionService   *session.Service
	sessionHandler   *sessionhandler.Handler
	vaultHandler     *vaulthandler.Handler
	itemHandler      *itemhandler.Handler
}

func NewApplication(
	cfg Config,
	logger *zap.SugaredLogger,
	readinessPinger health.Pinger,
	securityEnforcer SecurityEnforcer,
	authService authhandler.RegistrationService,
	sessionService *session.Service,
	vaultService vaulthandler.VaultService,
	itemService itemhandler.ItemService,
) *Application {
	sessionCookies := sessioncookie.NewManager(cfg.SessionCookies)

	return &Application{
		config:           cfg,
		logger:           logger,
		securityEnforcer: securityEnforcer,
		healthHandler: health.NewHealthCheckHandler(
			cfg.Env,
			readinessPinger,
		),
		authHandler: authhandler.New(
			authService,
			sessionService,
			securityEnforcer,
			sessionCookies,
			logger,
		),
		sessionService: sessionService,
		sessionHandler: sessionhandler.New(
			sessionService,
			sessionCookies,
			logger,
		),
		vaultHandler: vaulthandler.New(vaultService, logger),
		itemHandler:  itemhandler.New(itemService, logger),
	}
}
