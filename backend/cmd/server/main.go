package main

import (
	"log"

	"bongkey/backend/internal/config"
)

// main loads configuration and hands application startup to serve.
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration failed: %v", err)
	}
	if cfg.EmailTestCode != "" {
		log.Print("WARNING: email test mode is on: @example.test addresses get a fixed code (development only)")
	}
	if err := serve(cfg); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
