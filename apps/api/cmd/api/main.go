package main

import (
	"context"
	"log"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/api"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/auth"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/db"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/store"
)

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

	databaseContext, cancelDatabase := context.WithTimeout(
		context.Background(),
		5*time.Second,
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

	userStore := store.NewUserStore(
		databasePool,
	)
	passwordHasher := auth.NewArgon2idHasher()
	authService := auth.NewService(
		userStore,
		passwordHasher,
	)

	app := api.NewApplication(
		cfg,
		logger,
		databasePool,
		authService,
	)

	if err := app.Run(); err != nil {
		logger.Errorw(
			"application stopped unexpectedly",
			"error", err,
		)
	}
}
