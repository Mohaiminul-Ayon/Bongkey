package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bongkey/backend/internal/config"
	"bongkey/backend/internal/database"
	"bongkey/backend/internal/routes"
)

// shutdownGrace bounds how long in-flight requests such as checkout may finish after a stop signal.
const shutdownGrace = 15 * time.Second

// serve prepares application dependencies and blocks until the HTTP server stops.
func serve(cfg config.Config) error {
	logEnvironment(cfg)
	if err := database.Connect(cfg.DatabaseURL); err != nil {
		// PostgreSQL driver errors may contain credentials, so return a safe message only.
		return errors.New("database connection failed; check DATABASE_URL and PostgreSQL availability")
	}
	// The pool closes only after runHTTP returns, so draining requests can still reach the database.
	defer database.DB.Close()
	log.Println("database connected")

	if err := database.MigrateAndSeed(cfg); err != nil {
		return errors.New("database migration/seed failed; check database availability and migration state")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runHTTP(ctx, newHTTPServer(":"+cfg.ServerPort, routes.SetupRouter(cfg)))
}

// newHTTPServer bounds slow clients so a few idle or trickling connections cannot hold the server.
func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}

// runHTTP serves until ctx is cancelled, then stops new connections and drains in-flight requests.
func runHTTP(ctx context.Context, server *http.Server) error {
	serveErr := make(chan error, 1)
	// runHTTP owns this goroutine: it sends exactly one result and ends when ListenAndServe returns,
	// which happens on listen failure or after Shutdown below. The buffered channel never blocks it.
	go func() { serveErr <- server.ListenAndServe() }()
	select {
	case err := <-serveErr:
		return fmt.Errorf("listen on %s: %w", server.Addr, err)
	case <-ctx.Done():
	}
	log.Println("shutdown signal received; draining requests")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen on %s: %w", server.Addr, err)
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
