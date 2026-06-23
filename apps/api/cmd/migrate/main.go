package main

import (
	"errors"
	"log"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	migrator, err := migrate.New("file://migrations", databaseURL)
	if err != nil {
		log.Fatal("migration initialization failed")
	}

	defer func() {
		sourceErr, databaseErr := migrator.Close()
		if sourceErr != nil || databaseErr != nil {
			log.Print("migration resources did not close cleanly")
		}
	}()

	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatal("database migration failed")
	}

	log.Print("database migrations are current")
}
