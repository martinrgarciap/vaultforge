package main

import (
	"context"
	"log"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/api"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/health"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/auth"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/buildinfo"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/db"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/hashclient"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/hashpb"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/ratelimit"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/redisclient"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/store"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/telemetry"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

const dependencyStartupTimeout = 5 * time.Second

func main() {
	cfg, err := api.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	logger, err := api.NewLogger(cfg.Env)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		_ = logger.Sync()
	}()

	telemetryConfig, err := telemetry.LoadConfig()
	if err != nil {
		logger.Errorw(
			"telemetry configuration is invalid",
			"error", err,
		)

		return
	}

	telemetryContext, cancelTelemetry := context.WithTimeout(
		context.Background(),
		dependencyStartupTimeout,
	)

	shutdownTelemetry, err := telemetry.Start(
		telemetryContext,
		telemetryConfig,
		buildinfo.Current(),
		cfg.Env,
		logger,
	)

	cancelTelemetry()

	if err != nil {
		logger.Errorw(
			"telemetry initialization failed",
			"error", err,
		)

		return
	}

	defer func() {
		shutdownContext, cancel := context.WithTimeout(
			context.Background(),
			dependencyStartupTimeout,
		)
		defer cancel()

		if err := shutdownTelemetry(shutdownContext); err != nil {
			logger.Warnw(
				"OpenTelemetry shutdown failed",
			)
		}
	}()

	if telemetryConfig.Enabled() {
		logger.Infow(
			"OpenTelemetry tracing enabled",
		)
	}

	accessTokenManager, err := cfg.Tokens.NewAccessTokenManager()
	if err != nil {
		logger.Errorw(
			"access token manager initialization failed",
			"error", err,
		)

		return
	}

	databaseContext, cancelDatabase := context.WithTimeout(
		context.Background(),
		dependencyStartupTimeout,
	)

	databasePool, err := db.New(
		databaseContext,
		cfg.DatabaseURL,
	)

	cancelDatabase()

	if err != nil {
		logger.Errorw(
			"database initialization failed",
			"error", err,
		)

		return
	}

	defer databasePool.Close()

	logger.Infow(
		"database connection established",
	)

	redisContext, cancelRedis := context.WithTimeout(
		context.Background(),
		dependencyStartupTimeout,
	)

	redisClient, err := redisclient.New(
		redisContext,
		cfg.Redis,
	)

	cancelRedis()

	if err != nil {
		logger.Errorw(
			"Redis initialization failed",
			"error", err,
		)

		return
	}

	defer func() {
		if err := redisClient.Close(); err != nil {
			logger.Warnw("Redis client close failed")
		}
	}()

	logger.Infow(
		"Redis connection established",
	)

	rateLimitKeyBuilder, err :=
		cfg.RateLimits.NewKeyBuilder()
	if err != nil {
		logger.Errorw(
			"rate-limit key initialization failed",
		)

		return
	}

	securityEnforcer, err := ratelimit.NewEnforcer(
		redisClient,
		rateLimitKeyBuilder,
		cfg.RateLimits.LoginProtection,
	)
	if err != nil {
		logger.Errorw(
			"rate-limit initialization failed",
		)

		return
	}

	userStore := store.NewUserStore(
		databasePool,
	)

	hashServiceConnection, err := hashclient.Dial(
		context.Background(),
		cfg.HashService,
	)
	if err != nil {
		logger.Errorw(
			"hash service initialization failed",
			"error", err,
		)

		return
	}

	defer func() {
		if err := hashServiceConnection.Close(); err != nil {
			logger.Warnw("hash service connection close failed")
		}
	}()

	logger.Infow(
		"hash service connection established",
	)

	hashServiceClient := hashpb.NewHashServiceClient(
		hashServiceConnection,
	)

	passwordHasher, err := hashclient.New(
		hashServiceClient,
		cfg.HashService,
	)
	if err != nil {
		logger.Errorw(
			"password hasher initialization failed",
			"error", err,
		)

		return
	}

	hashServicePinger, err := hashclient.NewHealthPinger(
		hashServiceConnection,
		cfg.HashService,
	)
	if err != nil {
		logger.Errorw(
			"hash service readiness initialization failed",
			"error", err,
		)

		return
	}

	authService := auth.NewService(
		userStore,
		passwordHasher,
	)

	sessionStore := store.NewSessionStore(
		databasePool,
	)

	refreshTokenGenerator := session.NewRefreshTokenGenerator()

	sessionService := session.NewService(
		authService,
		sessionStore,
		refreshTokenGenerator,
		accessTokenManager,
		cfg.Tokens.Lifetimes(),
	)

	vaultStore := store.NewVaultStore(databasePool)
	vaultService := vault.NewService(vaultStore)

	readinessPinger := health.NewReadinessPinger(
		databasePool,
		redisClient,
		hashServicePinger,
	)

	app := api.NewApplication(
		cfg,
		logger,
		readinessPinger,
		securityEnforcer,
		authService,
		sessionService,
		vaultService,
		vaultService,
	)

	if err := app.Run(); err != nil {
		logger.Errorw(
			"application stopped unexpectedly",
			"error", err,
		)
	}
}
