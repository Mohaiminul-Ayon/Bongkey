package main

import (
	"log"

	"bongkey/backend/internal/config"
	"bongkey/backend/internal/database"
	"bongkey/backend/internal/routes"
)

// main connects application dependencies and starts the inherited HTTP server.
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
		log.Fatalf("database connection failed for DATABASE_URL=%q: %v", cfg.DatabaseURL, err)
	}
	log.Println("database connected")

	if err := database.MigrateAndSeed(cfg); err != nil {
		log.Fatalf("database migration/seed failed: %v", err)
	}

	router := routes.SetupRouter(cfg, database.DB)

	if err := router.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
