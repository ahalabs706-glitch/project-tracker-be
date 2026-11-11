package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"

	"projectflow-backend/internal/auth"
	"projectflow-backend/internal/config"
	"projectflow-backend/internal/database"
	"projectflow-backend/internal/httpapi"
	"projectflow-backend/internal/management"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	log, err := newLogger(cfg.Environment)
	if err != nil {
		return fmt.Errorf("initialize logger: %w", err)
	}
	defer func() {
		_ = log.Sync()
	}()

	if _, err := time.LoadLocation(cfg.Timezone); err != nil {
		return fmt.Errorf("load application timezone: %w", err)
	}

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get database connection: %w", err)
	}
	defer sqlDB.Close()

	authentication, err := auth.NewService(
		auth.NewPostgresStore(sqlDB),
		cfg.JWTSecret,
		cfg.AccessTTL,
		cfg.RefreshTTL,
	)
	if err != nil {
		return fmt.Errorf("initialize authentication: %w", err)
	}
	managementService, err := management.NewServiceWithTimezone(sqlDB, cfg.Timezone)
	if err != nil {
		return fmt.Errorf("initialize user management: %w", err)
	}

	server := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: httpapi.NewRouter(
			cfg.WebOrigin,
			sqlDB,
			log,
			authentication,
			managementService,
			strings.EqualFold(cfg.Environment, "production"),
		),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Info("starting ProjectFlow API", zap.String("address", server.Addr))
		serverErrors <- server.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}
	if err := <-serverErrors; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}

	log.Info("ProjectFlow API stopped")
	return nil
}

func newLogger(environment string) (*zap.Logger, error) {
	if environment == "production" {
		return zap.NewProduction()
	}
	return zap.NewDevelopment()
}
