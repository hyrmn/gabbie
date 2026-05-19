// Package main is the entrypoint for the todotracker web application.
package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hyrmn/todotracker/internal/auth"
	"github.com/hyrmn/todotracker/internal/db"
	"github.com/hyrmn/todotracker/internal/server"
	"github.com/hyrmn/todotracker/internal/templates"
)

//go:embed templates/*.html templates/components/* static/css/* static/js/* migrations/*.sql
var assets embed.FS

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logLevel := slog.LevelInfo
	if os.Getenv("DEBUG") == "1" {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// Supabase configuration
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseJWTSecret := os.Getenv("SUPABASE_JWT_SECRET")
	supabaseAnonKey := os.Getenv("SUPABASE_ANON_KEY")

	if supabaseURL == "" {
		logger.Warn("SUPABASE_URL not set, auth will not work in production", "note", "set env vars for Supabase integration")
	}

	// Initialize template engine
	tmpl, err := templates.New(assets)
	if err != nil {
		logger.Error("failed to load templates", "error", err)
		os.Exit(1)
	}

	// Initialize database
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "todotracker.db"
	}
	database, err := db.New(dbPath, assets, logger)
	if err != nil {
		logger.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	// Initialize auth service
	authSvc := auth.New(auth.Config{
		SupabaseURL:      supabaseURL,
		SupabaseJWTSecret: supabaseJWTSecret,
		SupabaseAnonKey:   supabaseAnonKey,
	}, database.DB, logger)

	// Start JWKS background refresh
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go authSvc.StartJWKSRefresh(ctx)

	mux := http.NewServeMux()

	// Static assets (embedded) — no StripPrefix needed since embed paths match URL paths
	mux.Handle("GET /static/", http.FileServer(http.FS(assets)))

	// Routes
	srv := server.New(mux, tmpl, logger, authSvc, database)
	srv.RegisterRoutes()

	addr := fmt.Sprintf(":%s", port)
	s := &http.Server{
		Addr:    addr,
		Handler: server.LoggingMiddleware(srv.Mux, logger),
	}

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("server starting", "addr", addr)
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-stop
	logger.Info("shutting down...")

	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()

	if err := s.Shutdown(ctx2); err != nil {
		logger.Error("forced shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped")
}
