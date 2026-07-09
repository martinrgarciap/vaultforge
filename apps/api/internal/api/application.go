package api

import (
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/authhandler"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/diagnostics"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/health"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/itemhandler"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/metrics"
	appmiddleware "github.com/martinrgarciap/vaultforge/apps/api/internal/api/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/passwordhandler"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/sessioncookie"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/sessionhandler"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/vaulthandler"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/buildinfo"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"

	"go.uber.org/zap"
)

type SecurityEnforcer interface {
	appmiddleware.RequestLimiter
	authhandler.LoginProtector
}

type Application struct {
	config             Config
	logger             *zap.SugaredLogger
	healthHandler      *health.Handler
	diagnosticsHandler *diagnostics.Handler
	metricsRegistry    *metrics.Registry
	securityEnforcer   SecurityEnforcer
	authHandler        *authhandler.Handler
	sessionService     *session.Service
	sessionHandler     *sessionhandler.Handler
	passwordHandler    *passwordhandler.Handler
	vaultHandler       *vaulthandler.Handler
	itemHandler        *itemhandler.Handler
}

func NewApplication(
	cfg Config,
	logger *zap.SugaredLogger,
	readinessPinger health.Pinger,
	securityEnforcer SecurityEnforcer,
	authService authhandler.RegistrationService,
	sessionService *session.Service,
	passwordService passwordhandler.Service,
	vaultService vaulthandler.VaultService,
	itemService itemhandler.ItemService,
) *Application {
	currentBuild := buildinfo.Current()
	metricsRegistry := metrics.New(currentBuild)
	sessionCookies := sessioncookie.NewManager(cfg.SessionCookies)

	return &Application{
		config:             cfg,
		logger:             logger,
		securityEnforcer:   securityEnforcer,
		diagnosticsHandler: diagnostics.New(currentBuild),
		metricsRegistry:    metricsRegistry,
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
		passwordHandler: passwordhandler.New(
			passwordService,
			logger,
		),
		vaultHandler: vaulthandler.New(vaultService, logger),
		itemHandler:  itemhandler.New(itemService, logger),
	}
}
