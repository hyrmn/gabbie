// Package server provides the HTTP server, router, and middleware.
package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/hyrmn/todotracker/internal/auth"
	"github.com/hyrmn/todotracker/internal/db"
	"github.com/hyrmn/todotracker/internal/handlers"
	"github.com/hyrmn/todotracker/internal/templates"
)

// Server holds the HTTP mux, templates, logger, DB, and auth service.
type Server struct {
	Mux     *http.ServeMux
	Tmpl    *templates.Engine
	Logger  *slog.Logger
	Handler *handlers.Handler
	Auth    *auth.Service
	DB      *db.DB
}

// New creates a new Server instance.
func New(mux *http.ServeMux, tmpl *templates.Engine, logger *slog.Logger, authService *auth.Service, database *db.DB) *Server {
	return &Server{
		Mux:    mux,
		Tmpl:   tmpl,
		Logger: logger,
		Handler: handlers.New(tmpl, logger, database),
		Auth:   authService,
		DB:     database,
	}
}

// RegisterRoutes sets up all HTTP routes.
func (s *Server) RegisterRoutes() {
	// Public routes
	authHandlers := s.Auth.NewHandlers(s.Tmpl.Render)
	s.Mux.HandleFunc("GET /login", authHandlers.Login)
	s.Mux.HandleFunc("POST /auth/callback", authHandlers.Callback)
	s.Mux.HandleFunc("POST /auth/logout", authHandlers.Logout)
	s.Mux.HandleFunc("GET /auth/session", authHandlers.Session)

	// Authenticated pages (wrapped in AuthMiddleware)
	protected := http.NewServeMux()
	protected.HandleFunc("GET /", s.Handler.Index)
	protected.HandleFunc("GET /dashboard", s.Handler.Dashboard)
	protected.HandleFunc("POST /lists", s.Handler.ListCreate)
	protected.HandleFunc("GET /lists/{id}", s.Handler.ListView)
	protected.HandleFunc("PUT /lists/{id}", s.Handler.ListUpdate)
	protected.HandleFunc("DELETE /lists/{id}", s.Handler.ListDelete)
	protected.HandleFunc("POST /lists/{id}/collaborators", s.Handler.AddCollaborator)
	protected.HandleFunc("DELETE /lists/{id}/collaborators/{userId}", s.Handler.RemoveCollaborator)

	// Item CRUD routes (behind AuthMiddleware)
	protected.HandleFunc("GET /lists/{id}/items", s.Handler.ListItems)
	protected.HandleFunc("POST /lists/{id}/items", s.Handler.CreateItem)
	protected.HandleFunc("PUT /items/{id}", s.Handler.UpdateItem)
	protected.HandleFunc("DELETE /items/{id}", s.Handler.DeleteItem)
	protected.HandleFunc("PUT /items/{id}/status", s.Handler.ToggleItemStatus)

	// Settings pages
	protected.HandleFunc("GET /settings", s.Handler.Settings)
	protected.HandleFunc("GET /settings/api-keys", s.Handler.SettingsAPIKeys)
	protected.HandleFunc("POST /settings/api-keys", s.Handler.CreateAPIKey)
	protected.HandleFunc("DELETE /settings/api-keys/{id}", s.Handler.RevokeAPIKey)

	s.Mux.Handle("/", AuthMiddleware(s.Auth, s.Logger)(protected))

	// API routes — accept both JWT cookie and API key auth
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /api/lists", s.Handler.ListsJSON)
	apiMux.HandleFunc("POST /api/lists", s.Handler.CreateListJSON)
	apiMux.HandleFunc("GET /api/lists/{id}", s.Handler.GetListJSON)
	apiMux.HandleFunc("POST /api/lists/{id}/items", s.Handler.CreateItemJSON)
	apiMux.HandleFunc("PUT /api/items/{id}", s.Handler.UpdateItemJSON)
	s.Mux.Handle("/api/", EitherAuthMiddleware(s.Auth, s.DB, s.Logger)(apiMux))
}

// LoggingMiddleware logs each request with method, path, duration, and status.
func LoggingMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lw, r)
		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", lw.statusCode,
			"duration", time.Since(start).String(),
		)
	})
}

// RecoveryMiddleware recovers from panics and returns 500.
func RecoveryMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("panic recovered", "error", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}
