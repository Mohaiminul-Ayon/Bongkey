package main

import (
	"errors"
	"fmt"
	"log"

	"bongkey/backend/internal/config"
	"bongkey/backend/internal/database"
	"bongkey/backend/internal/routes"
)

// serve prepares application dependencies and blocks while the HTTP server runs.
func serve(cfg config.Config) error {
	logEnvironment(cfg)
	if err := database.Connect(cfg.DatabaseURL); err != nil {
		// PostgreSQL driver errors may contain credentials, so return a safe message only.
		return errors.New("database connection failed; check DATABASE_URL and PostgreSQL availability")
	}
	defer database.DB.Close()
	log.Println("database connected")

	if err := database.MigrateAndSeed(cfg); err != nil {
		return errors.New("database migration/seed failed; check database availability and migration state")
	}
	if err := routes.SetupRouter(cfg).Run(":" + cfg.ServerPort); err != nil {
		return fmt.Errorf("listen on port %s: %w", cfg.ServerPort, err)
	}
	return nil
}

// logEnvironment reports where startup configuration came from without printing values.
func logEnvironment(cfg config.Config) {
	if cfg.EnvFile == "" {
		log.Printf("no .env file found. %s", cfg.EnvFileHint)
		return
	}
	log.Printf("loaded environment from %s", cfg.EnvFile)
}
