package main

import (
	"log"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/api"
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

	app := api.NewApplication(cfg, logger)

	if err := app.Run(); err != nil {
		logger.Errorw(
			"application stopped unexpectedly",
			"error", err,
		)
	}
}
