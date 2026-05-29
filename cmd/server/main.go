package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/codingdrone04/mail-service/internal/config"
	"github.com/codingdrone04/mail-service/internal/handler"
	"github.com/codingdrone04/mail-service/internal/mail"
	"github.com/codingdrone04/mail-service/internal/middleware"
)

func main() {
	// Structured JSON logging
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Load .env file (non-fatal if missing)
	if err := godotenv.Load(); err != nil {
		slog.Info("no .env file found, using environment variables")
	}

	cfg := config.Load()

	mailer := mail.NewMailer(cfg)
	sendHandler := handler.NewSendHandler(mailer)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health)
	mux.Handle("POST /send", middleware.RateLimit(cfg.RateLimitPerMin, http.HandlerFunc(sendHandler.Handle)))

	// Middleware stack: SecurityHeaders -> Logger -> CORS -> mux
	// SecurityHeaders is outermost so its headers land on every response,
	// including CORS preflights and rate-limit 429s.
	wrapped := middleware.SecurityHeaders(
		middleware.Logger(
			middleware.CORS(cfg.AllowedOrigins, mux),
		),
	)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      wrapped,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	slog.Info("server stopped")
}
