package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/M-Tier/webform-service/internal/config"
	"github.com/M-Tier/webform-service/internal/email"
	"github.com/M-Tier/webform-service/internal/handler"
	"github.com/M-Tier/webform-service/internal/security"
)

func main() {
	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Load sites configuration
	sites, err := config.LoadSites(cfg.SitesConfigPath)
	if err != nil {
		logger.Error("failed to load sites configuration", "error", err, "path", cfg.SitesConfigPath)
		os.Exit(1)
	}

	logger.Info("loaded sites configuration",
		"sites", sites.SiteIDs(),
		"origins", sites.AllOrigins(),
	)

	// Initialize dependencies
	emailSender := email.NewSender(cfg)
	rateLimiter := security.NewRateLimiter(cfg.RedisURL, cfg.RateLimitPerHour, logger)
	validator := security.NewValidator(cfg.MinFormTimeSeconds)

	// Create handlers
	contactHandler := handler.NewContactHandler(cfg, sites, emailSender, rateLimiter, validator, logger)
	healthHandler := handler.NewHealthHandler()

	// Setup routes
	mux := http.NewServeMux()
	mux.Handle("/api/contact", contactHandler)
	mux.Handle("/health", healthHandler)
	mux.Handle("/metrics", promhttp.Handler())

	// Create server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("starting server",
			"port", cfg.Port,
			"dev_mode", cfg.DevMode,
			"rate_limit_per_hour", cfg.RateLimitPerHour,
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	// Close rate limiter (closes Redis connection if open)
	if err := rateLimiter.Close(); err != nil {
		logger.Error("failed to close rate limiter", "error", err)
	}

	logger.Info("server stopped")
}
