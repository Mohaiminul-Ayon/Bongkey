package main

import (
	"log"

	"bongkey/backend/internal/config"
	"bongkey/backend/internal/database"
	"bongkey/backend/internal/routes"
)

// main starts the server only after configuration and database setup succeed.
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration failed: %v", err)
	}

	if cfg.EnvFile == "" {
		log.Printf("no .env file found. %s", cfg.EnvFileHint)
	} else {
		log.Printf("loaded environment from %s", cfg.EnvFile)
	}

	if err := database.Connect(cfg.DatabaseURL); err != nil {
		// Driver errors can include the connection URL, so do not log either value here.
		log.Fatal("database connection failed; check DATABASE_URL and PostgreSQL availability")
	}
	log.Println("database connected")

	if err := database.MigrateAndSeed(cfg); err != nil {
		log.Fatal("database migration/seed failed; check database availability and migration state")
	}

	router := routes.SetupRouter(cfg)

	if err := router.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
